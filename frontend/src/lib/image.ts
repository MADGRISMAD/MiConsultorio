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

/**
 * Shrinks a photo for the clinic's public page: the longest side at most `max` px, always JPEG (white behind transparency),
 * lowering the quality until it fits in about 1 MB, which is what the server accepts.
 */
export async function fileToPhoto(file: File, max = 1600): Promise<string> {
  if (!file.type.startsWith('image/')) throw new Error('Elige una imagen (JPG, PNG o WebP).');
  if (file.size > 25 * 1024 * 1024) throw new Error('La imagen pesa demasiado. Elige una de menos de 25 MB.');
  const bitmap = await createImageBitmap(file).catch(() => {
    throw new Error('No se pudo leer la imagen. Prueba con otra.');
  });
  const scale = Math.min(1, max / Math.max(bitmap.width, bitmap.height));
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(bitmap.width * scale));
  canvas.height = Math.max(1, Math.round(bitmap.height * scale));
  const ctx = canvas.getContext('2d')!;
  ctx.fillStyle = '#fff';
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
  bitmap.close?.();
  for (const q of [0.86, 0.76, 0.66, 0.55]) {
    const url = canvas.toDataURL('image/jpeg', q);
    if (url.length <= 1_300_000) return url;
  }
  throw new Error('La imagen sigue siendo muy pesada. Prueba con una más pequeña.');
}

/** A canvas as a JPEG data URL, lowering the quality until it fits in about 1 MB (what the server accepts for public photos). */
export function canvasToPhoto(canvas: HTMLCanvasElement): string {
  for (const q of [0.88, 0.78, 0.68, 0.56]) {
    const url = canvas.toDataURL('image/jpeg', q);
    if (url.length <= 1_300_000) return url;
  }
  throw new Error('La imagen sigue siendo muy pesada. Prueba con una más pequeña.');
}
