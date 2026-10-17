import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SectionPageLayout } from "@/components/layout";
import { useAuthStore } from "@/stores/auth-store";
import { ROLE } from "@/lib/roles";

type Provider = {
  id: number;
  name: string;
  model: string;
  base_url: string;
  protocol: string;
  input_price: number;
  output_price: number;
  max_output: number;
  enabled: boolean;
};
type Contribution = {
  id: number;
  provider_id: number;
  key_hint: string;
  enabled: boolean;
  daily_limit: number;
  daily_used: number;
  reserved: number;
};
type Charge = {
  request_id: string;
  direction: string;
  charged: number;
  status: string;
  created_at: string;
};
const usd = (quota: number) => "$" + (quota / 500_000).toFixed(5);
const inputStyle =
  "min-w-0 rounded-md border border-input bg-background px-3 py-2 text-sm";
const labelStyle = "text-sm font-medium";
const errorText = (err: unknown) => {
  if (err && typeof err === "object" && "response" in err) {
    const response = (
      err as { response?: { data?: { error?: { message?: string } } } }
    ).response;
    if (response?.data?.error?.message) return response.data.error.message;
  }
  return err instanceof Error ? err.message : "Request failed";
};
export function SharedPool() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [adminProviders, setAdminProviders] = useState<Provider[]>([]);
  const [priceEdits, setPriceEdits] = useState<
    Record<number, { input: string; output: string }>
  >({});
  const [contributions, setContributions] = useState<Contribution[]>([]);
  const [history, setHistory] = useState<Charge[]>([]);
  const [balance, setBalance] = useState(0);
  const [providerID, setProviderID] = useState("");
  const [key, setKey] = useState("");
  const [dailyLimit, setDailyLimit] = useState("1");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [grantUser, setGrantUser] = useState("");
  const [grantAmount, setGrantAmount] = useState("0.10");
  const [grantRef, setGrantRef] = useState("");
  const [form, setForm] = useState({
    name: "",
    base_url: "",
    protocol: "chat",
    model: "",
    input_price: "0.5",
    output_price: "2",
    max_output: "512",
  });
  const isAdmin =
    (useAuthStore((s) => s.auth.user?.role) ?? 0) >= ROLE.SUPER_ADMIN;
  const fetchData = useCallback(async () => {
    try {
      const [ps, mine] = await Promise.all([
        api.get("/api/shared/providers"),
        api.get("/api/shared/mine"),
      ]);
      setProviders(ps.data.data ?? []);
      setContributions(mine.data.data?.contributions ?? []);
      setHistory(mine.data.data?.history ?? []);
      setBalance(mine.data.data?.balance ?? 0);
      if (isAdmin) {
        const admin = await api.get("/api/shared/admin/providers");
        setAdminProviders(admin.data.data ?? []);
      }
    } catch (e) {
      setError(errorText(e));
    }
  }, [isAdmin]);
  useEffect(() => {
    void fetchData();
  }, [fetchData]);
  const mutate = async (action: () => Promise<unknown>, success: string) => {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      await action();
      setNotice(success);
      await fetchData();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>Token 共享中心</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className="mx-auto w-full max-w-5xl space-y-6 p-4">
          <section className="rounded-xl border p-5 space-y-2">
            <h2 className="text-lg font-semibold">
              原生钱包余额：{usd(balance)}
            </h2>
            <p className="text-sm text-muted-foreground">
              贡献自己的 API Key 后，当其他用户实际使用该 Key 时获得共享积分。
              奖励直接记入原生钱包，与现有 /v1 API 调用使用同一额度。
            </p>
            <p className="text-sm">
              共享 API 地址：
              <code className="select-all">https://api.nju-aia.com/v1</code>
            </p>
            <p className="text-sm">
              使用现有 API Token，兼容 OpenAI Chat
              Completions。支持文本请求与流式输出。
            </p>
            <p className="text-xs text-muted-foreground">
              请注意：共享资源池的上游服务可能处理和记录你发送的提示词。不要发送私密内容。
            </p>
          </section>
          {(error || notice) && (
            <div role="status" className="rounded-md border p-3 text-sm">
              {error || notice}
            </div>
          )}
          <section className="rounded-xl border p-5 space-y-4">
            <h2 className="text-lg font-semibold">贡献 API Key</h2>
            {providers.length === 0 && (
              <p className="text-sm text-muted-foreground">
                管理员尚未启用共享 Endpoint。
              </p>
            )}
            <form
              className="grid gap-3 md:grid-cols-2"
              onSubmit={(e) => {
                e.preventDefault();
                void mutate(async () => {
                  await api.post("/api/shared/contributions", {
                    provider_id: Number(providerID),
                    key,
                    daily_limit: Math.round(Number(dailyLimit) * 500_000),
                  });
                  setKey("");
                }, "Key 已通过实际模型调用验证并加入共享池");
              }}
            >
              <label className={labelStyle}>
                预设 Endpoint / 模型
                <select
                  className={"mt-1 w-full " + inputStyle}
                  required
                  value={providerID}
                  onChange={(e) => setProviderID(e.target.value)}
                >
                  <option value="">请选择</option>
                  {providers.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} — {p.model}
                    </option>
                  ))}
                </select>
              </label>
              <label className={labelStyle}>
                每日最大共享成本（美元）
                <Input
                  className="mt-1"
                  type="number"
                  min="0.0001"
                  max="100"
                  step="0.0001"
                  value={dailyLimit}
                  onChange={(e) => setDailyLimit(e.target.value)}
                  required
                />
              </label>
              <label className={labelStyle + " md:col-span-2"}>
                API Key（验证后密文保存）
                <Input
                  className="mt-1"
                  type="password"
                  autoComplete="off"
                  required
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                />
              </label>
              <div className="md:col-span-2">
                <Button disabled={busy || providers.length === 0} type="submit">
                  验证并贡献
                </Button>
              </div>
            </form>
          </section>
          <section className="rounded-xl border p-5 space-y-3">
            <h2 className="text-lg font-semibold">我的贡献</h2>
            {contributions.length === 0 && (
              <p className="text-sm text-muted-foreground">
                还没有贡献的 Key。
              </p>
            )}
            {contributions.map((keyItem) => (
              <div
                key={keyItem.id}
                className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-muted/40 p-3 text-sm"
              >
                <div>
                  <strong>
                    {providers.find((p) => p.id === keyItem.provider_id)
                      ?.model ?? "未启用模型"}
                  </strong>
                  <span className="ml-3">{keyItem.key_hint}</span>
                  <p className="text-muted-foreground">
                    今日已使用 {usd(keyItem.daily_used)} /{" "}
                    {usd(keyItem.daily_limit)}；预留 {usd(keyItem.reserved)}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button
                    disabled={busy}
                    variant="outline"
                    onClick={() =>
                      void mutate(
                        () =>
                          api.put("/api/shared/contributions/" + keyItem.id, {
                            enabled: !keyItem.enabled,
                            daily_limit: keyItem.daily_limit,
                          }),
                        "状态已更新",
                      )
                    }
                  >
                    {keyItem.enabled ? "暂停" : "启用"}
                  </Button>
                  <Button
                    disabled={busy}
                    variant="destructive"
                    onClick={() => {
                      if (
                        window.confirm(
                          "确定撤销此 Key？将永久销毁保存在平台的凭证。",
                        )
                      ) {
                        void mutate(
                          () =>
                            api.delete(
                              "/api/shared/contributions/" + keyItem.id,
                            ),
                          "凭证已撤销",
                        );
                      }
                    }}
                  >
                    撤销
                  </Button>
                </div>
              </div>
            ))}
          </section>
          <section className="rounded-xl border p-5 space-y-2">
            <h2 className="text-lg font-semibold">最近共享账单</h2>
            {history.length === 0 && (
              <p className="text-sm text-muted-foreground">暂无记录</p>
            )}
            {history.map((ch) => (
              <div
                className="flex flex-wrap justify-between gap-2 border-b py-2 text-sm"
                key={ch.request_id}
              >
                <span>
                  {(
                    {
                      consume: "消费",
                      earn: "贡献",
                      self: "自用",
                      grant: "赠送",
                    } as Record<string, string>
                  )[ch.direction] ?? ch.direction}{" "}
                  · {ch.status}
                </span>
                <span>{usd(ch.charged)}</span>
                <span className="text-muted-foreground">
                  {new Date(ch.created_at).toLocaleString()}
                </span>
              </div>
            ))}
          </section>
          {isAdmin && (
            <section className="rounded-xl border p-5 space-y-4">
              <div className="space-y-3 border-b pb-5">
                <h3 className="text-base font-semibold">初始共享积分</h3>
                <p className="text-sm text-muted-foreground">
                  未贡献 Key
                  的新用户可获管理员授予的少量启动额度。使用唯一流水标识防止重复发放。
                </p>
                <form
                  className="flex flex-wrap items-end gap-3"
                  onSubmit={(e) => {
                    e.preventDefault();
                    void mutate(
                      () =>
                        api.post("/api/shared/admin/grants", {
                          user_id: Number(grantUser),
                          quota: Math.round(Number(grantAmount) * 500_000),
                          reference: grantRef,
                        }),
                      "积分发放成功",
                    );
                  }}
                >
                  <label className={labelStyle}>
                    用户 ID
                    <Input
                      required
                      type="number"
                      min="1"
                      value={grantUser}
                      onChange={(e) => setGrantUser(e.target.value)}
                    />
                  </label>
                  <label className={labelStyle}>
                    美元等值积分
                    <Input
                      required
                      type="number"
                      step="0.000001"
                      min="0.000001"
                      max="100"
                      value={grantAmount}
                      onChange={(e) => setGrantAmount(e.target.value)}
                    />
                  </label>
                  <label className={labelStyle}>
                    唯一流水标识
                    <Input
                      required
                      maxLength={64}
                      value={grantRef}
                      onChange={(e) => setGrantRef(e.target.value)}
                    />
                  </label>
                  <Button disabled={busy} type="submit">
                    发放
                  </Button>
                </form>
              </div>
              <h2 className="text-lg font-semibold">
                管理员：预设共享 Endpoint
              </h2>
              <p className="text-xs text-muted-foreground">
                启用前请确认上游授权共享或代理，并核准模型定价。接口只接受 HTTPS
                /v1 端点。
              </p>
              <form
                className="grid grid-cols-1 gap-3 md:grid-cols-2"
                onSubmit={(e) => {
                  e.preventDefault();
                  void mutate(
                    () =>
                      api.post("/api/shared/admin/providers", {
                        ...form,
                        input_price: Number(form.input_price),
                        output_price: Number(form.output_price),
                        max_output: Number(form.max_output),
                      }),
                    "Endpoint 已添加，需手动启用",
                  );
                }}
              >
                <label className={labelStyle}>
                  上游协议
                  <select
                    className={"mt-1 w-full " + inputStyle}
                    value={form.protocol}
                    onChange={(e) =>
                      setForm((prev) => ({ ...prev, protocol: e.target.value }))
                    }
                  >
                    <option value="chat">OpenAI Chat Completions</option>
                    <option value="responses">OpenAI Responses</option>
                    <option value="anthropic">Anthropic Messages</option>
                    <option value="gemini">Gemini API</option>
                  </select>
                </label>
                {(
                  [
                    "name",
                    "base_url",
                    "model",
                    "input_price",
                    "output_price",
                    "max_output",
                  ] as const
                ).map((field) => (
                  <label key={field} className={labelStyle}>
                    {
                      {
                        name: "服务商名称",
                        base_url:
                          "HTTPS Endpoint（Gemini /v1beta/openai，其他 /v1）",
                        model: "模型名称",
                        input_price: "输入 USD / 百万 Token",
                        output_price: "输出 USD / 百万 Token",
                        max_output: "最大输出 Token",
                      }[field]
                    }
                    <Input
                      className="mt-1"
                      required
                      value={form[field]}
                      onChange={(e) =>
                        setForm((prev) => ({
                          ...prev,
                          [field]: e.target.value,
                        }))
                      }
                    />
                  </label>
                ))}
                <div>
                  <Button disabled={busy} type="submit">
                    添加预设 Endpoint
                  </Button>
                </div>
              </form>
              <p className="text-xs text-muted-foreground">
                官方 API 预设均默认停用。API Key 的额度与 OpenCode/Codex/Claude
                Code/Antigravity 客户端订阅、OAuth
                配额不同；启用前须确认服务条款和计费。
              </p>
              {adminProviders.map((p) => (
                <div key={p.id} className="space-y-2 border-t py-3 text-sm">
                  <p className="font-medium">
                    {p.name} — {p.model}（{p.enabled ? "已启用" : "未启用"}）
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {p.protocol || "chat"} · {p.base_url}
                  </p>
                  <div className="flex flex-wrap items-end gap-2">
                    <label>
                      输入 $/MTok
                      <Input
                        type="number"
                        min="0.000001"
                        step="0.000001"
                        className="w-28"
                        value={priceEdits[p.id]?.input ?? String(p.input_price)}
                        onChange={(e) =>
                          setPriceEdits((prev) => ({
                            ...prev,
                            [p.id]: {
                              input: e.target.value,
                              output:
                                prev[p.id]?.output ?? String(p.output_price),
                            },
                          }))
                        }
                      />
                    </label>
                    <label>
                      输出 $/MTok
                      <Input
                        type="number"
                        min="0.000001"
                        step="0.000001"
                        className="w-28"
                        value={
                          priceEdits[p.id]?.output ?? String(p.output_price)
                        }
                        onChange={(e) =>
                          setPriceEdits((prev) => ({
                            ...prev,
                            [p.id]: {
                              input: prev[p.id]?.input ?? String(p.input_price),
                              output: e.target.value,
                            },
                          }))
                        }
                      />
                    </label>
                    <Button
                      disabled={busy}
                      variant="outline"
                      onClick={() =>
                        void mutate(
                          () =>
                            api.put("/api/shared/admin/providers/" + p.id, {
                              enabled: p.enabled,
                              input_price: Number(
                                priceEdits[p.id]?.input ?? p.input_price,
                              ),
                              output_price: Number(
                                priceEdits[p.id]?.output ?? p.output_price,
                              ),
                            }),
                          "价格已保存",
                        )
                      }
                    >
                      保存价格
                    </Button>
                    <Button
                      disabled={busy}
                      variant="outline"
                      onClick={() =>
                        void mutate(
                          () =>
                            api.put("/api/shared/admin/providers/" + p.id, {
                              enabled: !p.enabled,
                              input_price: Number(
                                priceEdits[p.id]?.input ?? p.input_price,
                              ),
                              output_price: Number(
                                priceEdits[p.id]?.output ?? p.output_price,
                              ),
                            }),
                          "Endpoint 状态已更新",
                        )
                      }
                    >
                      {p.enabled ? "停用" : "启用"}
                    </Button>
                  </div>
                </div>
              ))}
            </section>
          )}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  );
}
