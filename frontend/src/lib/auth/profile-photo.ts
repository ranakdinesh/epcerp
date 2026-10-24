/** Decode locally, apply camera orientation, centre-crop and compress before upload.
 * Original files and their metadata never leave the browser. The API still validates
 * the resulting raster independently.
 */
export async function prepareProfilePhoto(file: Blob): Promise<Blob> {
  const url = URL.createObjectURL(file);
  const image = new Image();
  try {
    image.src = url;
    await image.decode();
    if (!image.naturalWidth || !image.naturalHeight) throw new Error("Empty image");
    const canvas = document.createElement("canvas");
    canvas.width = canvas.height = 512;
    const context = canvas.getContext("2d");
    if (!context) throw new Error("Image processing unavailable");
    const side = Math.min(image.naturalWidth, image.naturalHeight);
    // A white base avoids black backgrounds when a transparent PNG becomes JPEG.
    context.fillStyle = "#fff";
    context.fillRect(0, 0, canvas.width, canvas.height);
    context.drawImage(image, (image.naturalWidth - side) / 2, (image.naturalHeight - side) / 2, side, side, 0, 0, canvas.width, canvas.height);
    const photo = await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob(blob => blob ? resolve(blob) : reject(new Error("Image encoding failed")), "image/jpeg", 0.85);
    });
    if (photo.size > 512 * 1024) throw new Error("Image encoding failed");
    return photo;
  } catch {
    throw new Error("We couldn’t prepare this photo. Try another image in a format your browser supports, such as JPG, PNG or WebP.");
  } finally {
    image.src = "";
    URL.revokeObjectURL(url);
  }
}
