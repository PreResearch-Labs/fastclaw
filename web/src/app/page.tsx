"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getStatus } from "@/lib/api";
import { getMe } from "@/lib/auth";

export default function RootRedirect() {
  const router = useRouter();
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    Promise.all([getStatus(), getMe()])
      .then(([status, user]) => {
        if (!user) {
          router.replace("/login");
          return;
        }
        if (status.configured) {
          router.replace("/overview/");
        } else {
          router.replace("/onboard/");
        }
      })
      .catch(() => {
        router.replace("/login");
      })
      .finally(() => setChecking(false));
  }, [router]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-950">
      <div className="h-8 w-8 animate-spin rounded-full border-2 border-zinc-700 border-t-violet-500" />
    </div>
  );
}