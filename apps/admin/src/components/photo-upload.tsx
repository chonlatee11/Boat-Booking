'use client';

import { useState } from 'react';
import { UploadIcon, XIcon } from 'lucide-react';
import { rpc } from '@/lib/api';
import type { PresignPierPhotoResponseJson } from '@gen/services/catalog/v1/catalog_pb';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';

const MAX_PHOTO_BYTES = 5 * 1024 * 1024;
const ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/webp'];
const VALIDATION_ERROR = 'รองรับเฉพาะไฟล์ JPEG/PNG/WebP ขนาดไม่เกิน 5MB';

/**
 * Type/size validation → PresignPierPhoto → direct browser PUT to object
 * storage → photoKey (D-19). catalog never sees the file bytes.
 */
export function PhotoUpload({
  photoKey,
  photoUrl,
  onChange,
}: {
  photoKey: string;
  photoUrl?: string;
  onChange: (photoKey: string) => void;
}) {
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);

  const displayUrl = previewUrl ?? (photoKey ? photoUrl : undefined);

  async function handleFile(file: File | undefined) {
    if (!file) return;
    setError(null);
    if (
      !ALLOWED_TYPES.includes(file.type) ||
      file.size < 1 ||
      file.size > MAX_PHOTO_BYTES
    ) {
      setError(VALIDATION_ERROR);
      return;
    }
    setUploading(true);
    try {
      const presign = await rpc<PresignPierPhotoResponseJson>(
        'catalog',
        'PresignPierPhoto',
        { contentType: file.type, sizeBytes: String(file.size) },
      );
      if (!presign.uploadUrl || !presign.photoKey) {
        throw new Error('presign response missing uploadUrl/photoKey');
      }
      const putRes = await fetch(presign.uploadUrl, {
        method: 'PUT',
        headers: { 'Content-Type': file.type },
        body: file,
      });
      if (!putRes.ok) {
        throw new Error(`upload PUT failed with status ${putRes.status}`);
      }
      setPreviewUrl(URL.createObjectURL(file));
      onChange(presign.photoKey);
    } catch {
      setError('อัปโหลดรูปภาพไม่สำเร็จ กรุณาลองใหม่');
    } finally {
      setUploading(false);
    }
  }

  function handleRemove() {
    setPreviewUrl(null);
    setError(null);
    onChange('');
  }

  return (
    <div className="flex flex-col gap-2">
      {displayUrl ? (
        <div className="relative w-fit">
          {/* eslint-disable-next-line @next/next/no-img-element -- external/blob URL, no next/image loader configured for it */}
          <img
            src={displayUrl}
            alt=""
            className="h-32 w-32 rounded-lg border object-cover"
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="absolute -top-3 -right-3 size-11 rounded-full border bg-background"
            aria-label="ลบรูปภาพ"
            onClick={handleRemove}
          >
            <XIcon className="size-4" />
          </Button>
        </div>
      ) : (
        <label className="flex h-32 w-32 cursor-pointer flex-col items-center justify-center gap-1 rounded-lg border border-dashed text-sm text-muted-foreground">
          {uploading ? <Spinner /> : <UploadIcon className="size-5" />}
          <span>อัปโหลดรูปภาพ</span>
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            className="sr-only"
            disabled={uploading}
            onChange={(e) => {
              void handleFile(e.target.files?.[0]);
              e.target.value = '';
            }}
          />
        </label>
      )}
      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  );
}
