import { expect, test } from '../support/app';

// A thermal printer on a serial port receives the designed ticket as an ESC/POS image, not plain text lines.
test('la prueba de impresión por puerto serie manda el ticket como imagen', async ({ page }) => {
  await page.addInitScript(() => {
    const chunks: number[][] = [];
    (window as any).__printed = chunks;
    const port = {
      open: async () => {},
      close: async () => {},
      readable: null,
      writable: { getWriter: () => ({ write: async (b: Uint8Array) => void chunks.push(Array.from(b)), releaseLock() {} }) }
    };
    Object.defineProperty(navigator, 'serial', { value: { requestPort: async () => port, getPorts: async () => [port] }, configurable: true });
  });
  await page.goto('/ajustes?s=impresora');
  await page.getByText('Térmica por puerto serie').click();
  await page.getByRole('button', { name: 'Conectar impresora' }).click();
  await expect(page.getByText('Impresora conectada').first()).toBeVisible();
  await page.getByRole('button', { name: 'Imprimir prueba' }).click();
  await expect(page.getByText('Prueba enviada').first()).toBeVisible();

  const bytes: number[] = (await page.evaluate(() => (window as any).__printed)).flat();
  // ESC @ (init), then GS v 0 (raster image): a plain-text ticket would not contain the raster command
  expect(bytes.slice(0, 2)).toEqual([0x1b, 0x40]);
  const hasRaster = bytes.some((b, i) => b === 0x1d && bytes[i + 1] === 0x76 && bytes[i + 2] === 0x30);
  expect(hasRaster).toBe(true);
  expect(bytes.length).toBeGreaterThan(5000);
});
