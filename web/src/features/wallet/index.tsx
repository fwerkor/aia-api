import { useEffect, useState } from "react";
import { SectionPageLayout } from "@/components/layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getSelf } from "@/lib/api";
import { formatQuota } from "@/lib/format";
import { api } from "@/lib/api";

type Balance = { quota: number; used_quota: number };

export function Wallet(_props: { initialShowHistory?: boolean }) {
  const [balance, setBalance] = useState<Balance | null>(null);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const refresh = async () => {
    const response = await getSelf();
    if (response.success && response.data) setBalance(response.data as Balance);
  };

  useEffect(() => {
    void refresh();
  }, []);

  const redeem = async () => {
    if (!code.trim() || busy) return;
    setBusy(true);
    setMessage("");
    try {
      const response = await api.post("/api/user/topup", { key: code.trim() });
      if (!response.data?.success) {
        throw new Error(response.data?.message || "兑换失败");
      }
      setCode("");
      setMessage("兑换成功");
      await refresh();
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "兑换失败");
    } finally {
      setBusy(false);
    }
  };

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>我的额度</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className="mx-auto max-w-3xl space-y-6 p-4">
          <section className="rounded-xl border p-5 space-y-2">
            <h2 className="text-xl font-semibold">可用余额</h2>
            <div className="text-3xl font-semibold">
              {balance ? formatQuota(balance.quota) : "—"}
            </div>
            <p className="text-sm text-muted-foreground">
              累计使用 {balance ? formatQuota(balance.used_quota) : "—"}
            </p>
          </section>
          <section className="rounded-xl border p-5 space-y-4">
            <h2 className="text-lg font-semibold">兑换码兑换额度</h2>
            <form
              className="flex flex-col gap-3 sm:flex-row"
              onSubmit={(e) => {
                e.preventDefault();
                void redeem();
              }}
            >
              <Input
                aria-label="兑换码"
                placeholder="输入兑换码"
                value={code}
                onChange={(e) => setCode(e.target.value)}
                required
              />
              <Button disabled={busy || !code.trim()} type="submit">
                {busy ? "兑换中" : "兑换"}
              </Button>
            </form>
            {message && (
              <p role="status" className="text-sm">
                {message}
              </p>
            )}
          </section>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  );
}
