import type { FileKind } from '$lib/types/files';

export const KINDS: { value: FileKind; label: string }[] = [
  { value: 'xray', label: 'Radiografía' },
  { value: 'lab', label: 'Laboratorio' },
  { value: 'ultrasound', label: 'Ultrasonido' },
  { value: 'consent', label: 'Consentimiento' },
  { value: 'photo', label: 'Fotografía' },
  { value: 'document', label: 'Documento' },
  { value: 'other', label: 'Otro' }
];

export const kindLabel = (k: string) => KINDS.find((x) => x.value === k)?.label ?? k;

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  if (n < 1024 ** 3) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  return `${(n / 1024 ** 3).toFixed(2)} GB`;
}

export const isImage = (mime: string) => mime.startsWith('image/');
export const isPdf = (mime: string) => mime === 'application/pdf';
export const isDicom = (mime: string) => mime === 'application/dicom';
export const canPreview = (mime: string) => isImage(mime) || isPdf(mime) || isDicom(mime);

export const ACCEPT = 'image/jpeg,image/png,image/webp,image/gif,application/pdf,.dcm,.dicom,application/dicom';
