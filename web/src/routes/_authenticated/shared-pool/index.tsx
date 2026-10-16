import { createFileRoute } from "@tanstack/react-router";
import { SharedPool } from "@/features/shared-pool";

export const Route = createFileRoute("/_authenticated/shared-pool/")({
  component: SharedPool,
});
