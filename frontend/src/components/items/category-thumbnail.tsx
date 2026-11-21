"use client";

import Image from "next/image";
import { useState } from "react";

const illustrations: Record<string, string> = {
  CAB: "/purchase-thumbnails/ht-cable.webp",
  LED: "/purchase-thumbnails/flood-light.webp",
};

export function CategoryThumbnail({ code, revision = 0 }: { code?: string | null; revision?: number }) {
  const [failedSource, setFailedSource] = useState("");
  const source = code ? `/api/category-images/${encodeURIComponent(code)}?v=${revision}` : "";
  const image = source && failedSource !== source ? source : code ? illustrations[code] : undefined;
  if (!image) return <span aria-hidden="true" className="size-12 shrink-0 rounded-lg bg-slate-200 dark:bg-slate-700" />;
  return <Image src={image} alt="" width={48} height={48} loading="lazy" unoptimized
    className="size-12 shrink-0 rounded-lg bg-slate-100 object-contain"
    onError={() => setFailedSource(source)} />;
}
