/** Shrinks a chosen picture to a small data URL (the clinic's logo or photo): at most 256 px, PNG when it was a PNG, otherwise JPEG. */
export async function fileToLogo(file: File, max = 256): Promise<string> {
  if (!file.type.startsWith('image/')) throw new Error('Elige una imagen (PNG, JPG o WebP).');
  if (file.size > 8 * 1024 * 1024) throw new Error('La imagen pesa demasiado. Elige una de menos de 8 MB.');
  const bitmap = await createImageBitmap(file).catch(() => {
    throw new Error('No se pudo leer la imagen. Prueba con otra.');
  });
  const scale = Math.min(1, max / Math.max(bitmap.width, bitmap.height));
  const w = Math.max(1, Math.round(bitmap.width * scale));
  const h = Math.max(1, Math.round(bitmap.height * scale));
  const canvas = document.createElement('canvas');
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext('2d')!;
  const png = file.type === 'image/png';
  if (!png) {
    ctx.fillStyle = '#fff';
    ctx.fillRect(0, 0, w, h);
  }
  ctx.drawImage(bitmap, 0, 0, w, h);
  bitmap.close?.();
  const url = png ? canvas.toDataURL('image/png') : canvas.toDataURL('image/jpeg', 0.88);
  if (url.length > 280_000) throw new Error('La imagen sigue siendo muy pesada. Prueba con una más sencilla.');
  return url;
}
