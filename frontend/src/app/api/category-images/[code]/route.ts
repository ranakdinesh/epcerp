import { cookies } from "next/headers";
import sharp from "sharp";
import { AUTH_COOKIE_NAMES } from "@/lib/auth/session";
import { hasSameOrigin } from "@/lib/auth/request-origin";
import { requireAnyPermission } from "@/server/auth/access";
import { transaction } from "@/server/db";
import { AppError } from "@/server/errors";

const permissions = ["settings.categories.read", "settings.categories.write", "projects.purchase_bo.read", "projects.items.read", "settings.items.read"];
const maxBytes = 1024 * 1024;
type Context = { params: Promise<{ code: string }> };

async function handle(request: Request, { params }: Context) {
  const token = (await cookies()).get(AUTH_COOKIE_NAMES.accessToken)?.value;
  if (!token) return new Response(null, { status: 401 });
  const { code } = await params;
  if (!/^[A-Z0-9]{1,32}$/.test(code)) return new Response(null, { status: 404 });
  const upload = request.method === "PUT";
  if (upload && !hasSameOrigin(request)) return new Response(null, { status: 403 });

  try {
    if (!upload) return await transaction(async db => {
      await requireAnyPermission(db, token, permissions);
      const { rows: [row] } = await db.query<{ image: Buffer }>(
        "SELECT i.image FROM category_images i JOIN categories c ON c.id=i.category_id WHERE c.code=$1", [code]);
      return row ? new Response(new Uint8Array(row.image), { headers: {
        "Content-Type": "image/webp", "Cache-Control": "private, no-cache", "X-Content-Type-Options": "nosniff",
      } }) : new Response(null, { status: 404 });
    });

    await transaction(db => requireAnyPermission(db, token, ["settings.categories.write"]));
    const contentType = request.headers.get("content-type");
    if (!contentType || !["image/png", "image/jpeg", "image/webp"].includes(contentType))
      return Response.json({ detail: "Choose a PNG, JPEG or WebP image." }, { status: 415 });
    const reader = request.body?.getReader();
    if (!reader) return new Response(null, { status: 400 });
    const chunks: Uint8Array[] = [];
    let size = 0;
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        size += value.byteLength;
        if (size > maxBytes) {
          await reader.cancel();
          return Response.json({ detail: "Choose an image up to 1 MB." }, { status: 413 });
        }
        chunks.push(value);
      }
    } finally {
      reader.releaseLock();
    }
    let image: Buffer;
    try {
      const source = sharp(Buffer.concat(chunks), { limitInputPixels: 16_000_000 });
      const { format } = await source.metadata();
      if (!format || !["jpeg", "png", "webp"].includes(format)) throw new Error("Unsupported format");
      image = await source.rotate().resize(256, 256, { fit: "inside", withoutEnlargement: true }).webp().toBuffer();
    } catch {
      return Response.json({ detail: "The image could not be read. Choose a valid PNG, JPEG or WebP." }, { status: 422 });
    }
    return await transaction(async db => {
      const actor = await requireAnyPermission(db, token, ["settings.categories.write"], "business");
      const { rowCount } = await db.query(`
        INSERT INTO category_images(category_id,image,updated_by_id)
        SELECT id,$2,$3 FROM categories WHERE code=$1
        ON CONFLICT (category_id) DO UPDATE
        SET image=excluded.image,updated_by_id=excluded.updated_by_id,updated_at=now()
      `, [code, image, actor.id]);
      return new Response(null, { status: rowCount ? 204 : 404 });
    });
  } catch (error) {
    if (error instanceof AppError) return Response.json({ detail: error.message }, { status: error.status });
    throw error;
  }
}

export const GET = handle;
export const PUT = handle;
