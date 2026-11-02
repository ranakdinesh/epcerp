/** File identity checks must also work on development HTTP IP origins. */
export async function fileSHA256(file: Blob): Promise<string> {
  if (globalThis.crypto?.subtle) {
    const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
    return Array.from(new Uint8Array(digest), value => value.toString(16).padStart(2, "0")).join("");
  }

  // Load the portable implementation only when WebCrypto is unavailable.
  // Chunking bounds memory and yields between chunks for large tender files.
  const { sha256 } = await import("@noble/hashes/sha2.js");
  const hash = sha256.create();
  const chunkSize = 1024 * 1024;
  for (let offset = 0; offset < file.size; offset += chunkSize) {
    hash.update(new Uint8Array(await file.slice(offset, offset + chunkSize).arrayBuffer()));
    await new Promise(resolve => setTimeout(resolve, 0));
  }
  return Array.from(hash.digest(), value => value.toString(16).padStart(2, "0")).join("");
}
