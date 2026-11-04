import "server-only";
import { createCipheriv, createDecipheriv, createHash, randomBytes, timingSafeEqual } from "node:crypto";
import sharp from "sharp";
import { S3Client, HeadBucketCommand, CreateBucketCommand, PutObjectCommand, GetObjectCommand } from "@aws-sdk/client-s3";
import { AppError } from "../../errors";
import { fieldError } from "../catalogue/validation";
export const maximumBytes = 15 * 1024 * 1024;
export const integrity = () => new AppError(409, "Document integrity verification failed");
function key() {
    const value = process.env.EPCERP_SUPPLIER_DOCUMENT_KEY ?? "";
    if (!/^[a-f0-9]{64}$/i.test(value))
        throw new Error("Configure supplier document encryption key");
    return Buffer.from(value, "hex");
}
export const hash = (data: Buffer) => createHash("sha256").update(data).digest("hex");
export function seal(data: Buffer) {
    const nonce = randomBytes(12), cipher = createCipheriv("aes-256-gcm", key(), nonce);
    return { encrypted: Buffer.concat([nonce, cipher.update(data), cipher.final(), cipher.getAuthTag()]), sha256: hash(data) };
}
export function open(data: Buffer, expectedHash: string) {
    try {
        if (data.length < 28 || data.length > maximumBytes + 28 || !/^[a-f0-9]{64}$/.test(expectedHash))
            throw integrity();
        const cipher = createDecipheriv("aes-256-gcm", key(), data.subarray(0, 12));
        cipher.setAuthTag(data.subarray(-16));
        const plain = Buffer.concat([cipher.update(data.subarray(12, -16)), cipher.final()]);
        if (!timingSafeEqual(Buffer.from(hash(plain), "hex"), Buffer.from(expectedHash, "hex")))
            throw integrity();
        return plain;
    }
    catch {
        throw integrity();
    }
}
export async function inspect(data: Buffer) {
    if (!data.length || data.length > maximumBytes)
        fieldError("file", "File exceeds 15 MiB");
    if (data.subarray(0, 5).toString() === "%PDF-" && data.subarray(-2048).includes(Buffer.from("%%EOF")))
        return "application/pdf";
    const invalid = () => fieldError("file", "Only valid PDF, PNG, JPEG or GIF files are allowed");
    // Reject vector/other decoders before invoking libvips; then fully decode the first image.
    const known = data.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])) || data.subarray(0, 3).equals(Buffer.from([255, 216, 255])) || /^GIF8[79]a$/.test(data.subarray(0, 6).toString());
    if (!known)
        invalid();
    const image = sharp(data, { failOn: "warning", limitInputPixels: 40000000 });
    let metadata;
    try {
        metadata = await image.metadata();
    }
    catch {
        return invalid();
    }
    if (!metadata.width || !metadata.height || metadata.width > 16000 || metadata.height > 16000 || metadata.width * metadata.height > 40000000 || !["png", "jpeg", "gif"].includes(metadata.format!))
        invalid();
    try {
        await image.raw().toBuffer();
    }
    catch {
        fieldError("file", "Image is corrupt");
    }
    return { png: "image/png", jpeg: "image/jpeg", gif: "image/gif" }[metadata.format as "png" | "jpeg" | "gif"];
}
function storage() {
    const host = process.env.VTPL_STORAGE_ENDPOINT ?? "", secure = process.env.VTPL_STORAGE_SECURE === "true";
    const endpoint = new URL(`${secure ? "https" : "http"}://${host}`);
    if (!host || endpoint.username || endpoint.password || endpoint.pathname !== "/" || endpoint.search || endpoint.hash)
        throw new Error("Invalid document storage endpoint");
    return new S3Client({ endpoint: endpoint.origin, region: "us-east-1", forcePathStyle: true, maxAttempts: 2,
        credentials: { accessKeyId: process.env.VTPL_STORAGE_ACCESS_KEY ?? "", secretAccessKey: process.env.VTPL_STORAGE_SECRET_KEY ?? "" },
        requestHandler: { connectionTimeout: 5000, requestTimeout: 30000, throwOnRequestTimeout: true, socketTimeout: 30000 } });
}
export const documentBucket = () => { const bucket = process.env.VTPL_STORAGE_BUCKET; if (!bucket)
    throw new Error("Configure document bucket"); return bucket; };
export async function putObject(bucket: string, key: string, encrypted: Buffer) {
    const client = storage();
    try {
        try {
            await client.send(new HeadBucketCommand({ Bucket: bucket }));
        }
        catch (e) {
            if ((e as {
                $metadata?: {
                    httpStatusCode: number;
                };
            }).$metadata?.httpStatusCode !== 404)
                throw e;
            try {
                await client.send(new CreateBucketCommand({ Bucket: bucket }));
            }
            catch (e) {
                if (!["BucketAlreadyOwnedByYou", "BucketAlreadyExists"].includes((e as Error).name))
                    throw e;
            }
        }
        await client.send(new PutObjectCommand({ Bucket: bucket, Key: key, Body: encrypted, ContentType: "application/octet-stream" }));
    }
    finally {
        client.destroy();
    }
}
export async function getObject(bucket: string, key: string) {
    const client = storage();
    try {
        const response = await client.send(new GetObjectCommand({ Bucket: bucket, Key: key }));
        const body = response.Body as import("node:stream").Readable | undefined;
        if (!body)
            throw integrity();
        const chunks: Buffer[] = [];
        let size = 0;
        try {
            for await (const chunk of body) {
                size += chunk.length;
                if (size > maximumBytes + 28)
                    throw integrity();
                chunks.push(Buffer.from(chunk));
            }
        }
        finally {
            body.destroy();
        }
        return Buffer.concat(chunks);
    }
    catch {
        throw integrity();
    }
    finally {
        client.destroy();
    }
}
