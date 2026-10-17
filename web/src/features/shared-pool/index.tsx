import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SectionPageLayout } from "@/components/layout";
import { useAuthStore } from "@/stores/auth-store";
import { ROLE } from "@/lib/roles";

type CatalogModel = {
  id: number;
  code: string;
  name: string;
  protocol: string;
  input_price: number;
  output_price: number;
  max_output: number;
  enabled: boolean;
  enabled_on_provider?: boolean;
};
type Provider = {
  id: number;
  name: string;
  models: CatalogModel[];
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
  models: string;
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
  const [catalog, setCatalog] = useState<CatalogModel[]>([]);
  const [selectedModels, setSelectedModels] = useState<string[]>([]);
  const [modelToBind, setModelToBind] = useState<Record<number, string>>({});
  const [adminProviders, setAdminProviders] = useState<Provider[]>([]);
  const [adminCatalog, setAdminCatalog] = useState<CatalogModel[]>([]);
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
  });
  const [newModel, setNewModel] = useState({
    code: "",
    name: "",
    protocol: "chat",
    input_price: "0",
    output_price: "0",
    max_output: "2048",
  });
  const [modelEdits, setModelEdits] = useState<
    Record<number, { input: string; output: string }>
  >({});
  const isAdmin =
    (useAuthStore((s) => s.auth.user?.role) ?? 0) >= ROLE.SUPER_ADMIN;
  const fetchData = useCallback(async () => {
    try {
      const [ps, mine, catalogRes] = await Promise.all([
        api.get("/api/shared/providers"),
        api.get("/api/shared/mine"),
        api.get("/api/shared/models"),
      ]);
      setCatalog(catalogRes.data.data ?? []);
      setProviders(ps.data.data ?? []);
      setContributions(mine.data.data?.contributions ?? []);
      setHistory(mine.data.data?.history ?? []);
      setBalance(mine.data.data?.balance ?? 0);
      if (isAdmin) {
        const [admin, adminModels] = await Promise.all([
          api.get("/api/shared/admin/providers"),
          api.get("/api/shared/admin/models"),
        ]);
        setAdminProviders(admin.data.data ?? []);
        setAdminCatalog(adminModels.data.data ?? []);
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
          <section className="rounded-xl border p-5 space-y-3">
            <h2 className="text-lg font-semibold">模型目录</h2>
            <p className="text-sm text-muted-foreground">
              模型独立于上游
              Endpoint；被绑定且有可用渠道后才可调用。价格为参考配置，未核价显示“待核价”。
            </p>
            <div className="grid gap-2 md:grid-cols-2">
              {catalog.map((m) => (
                <div key={m.id} className="rounded-lg bg-muted/40 p-3 text-sm">
                  <div className="font-medium">{m.name}</div>
                  <div className="text-muted-foreground">
                    {m.code} · {m.protocol}
                  </div>
                  <div className="text-muted-foreground">
                    {m.input_price > 0 && m.output_price > 0
                      ? "$" +
                        m.input_price +
                        " / $" +
                        m.output_price +
                        " / MTok"
                      : "待核价"}
                  </div>
                </div>
              ))}
            </div>
          </section>
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
                    models: selectedModels,
                    key,
                    daily_limit: Math.round(Number(dailyLimit) * 500_000),
                  });
                  setKey("");
                  setSelectedModels([]);
                }, "Key 已通过实际模型调用验证并加入共享池");
              }}
            >
              <label className={labelStyle}>
                上游 Endpoint
                <select
                  className={"mt-1 w-full " + inputStyle}
                  required
                  value={providerID}
                  onChange={(e) => {
                    setProviderID(e.target.value);
                    setSelectedModels([]);
                  }}
                >
                  <option value="">请选择</option>
                  {providers.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </label>
              <div className="md:col-span-2 space-y-2">
                <p className={labelStyle}>
                  允许该 Key 贡献的模型（可多选，逐个实际调用验证）
                </p>
                {(
                  providers.find((p) => String(p.id) === providerID)?.models ??
                  []
                ).map((m) => (
                  <label key={m.id} className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={selectedModels.includes(m.code)}
                      onChange={(e) =>
                        setSelectedModels((prev) =>
                          e.target.checked
                            ? [...prev, m.code]
                            : prev.filter((id) => id !== m.code),
                        )
                      }
                    />
                    {m.name}{" "}
                    <span className="text-muted-foreground">({m.code})</span>
                  </label>
                ))}
                {providerID &&
                  !providers.find((p) => String(p.id) === providerID)?.models
                    .length && (
                    <p className="text-sm text-muted-foreground">
                      该 Endpoint 尚未绑定模型。
                    </p>
                  )}
              </div>
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
                <Button
                  disabled={
                    busy ||
                    providers.length === 0 ||
                    selectedModels.length === 0
                  }
                  type="submit"
                >
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
                  <strong>{keyItem.models || "未选择模型"}</strong>
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
              <h2 className="text-lg font-semibold">管理员：独立模型目录</h2>
              <p className="text-xs text-muted-foreground">
                模型独立维护 API
                ID、协议、参考价格和输出上限；模型目录并不保证任一上游都支持该模型。
              </p>
              <form
                className="grid gap-3 md:grid-cols-3"
                onSubmit={(e) => {
                  e.preventDefault();
                  void mutate(
                    () =>
                      api.post("/api/shared/admin/models", {
                        ...newModel,
                        input_price: Number(newModel.input_price),
                        output_price: Number(newModel.output_price),
                        max_output: Number(newModel.max_output),
                      }),
                    "模型已加入目录",
                  );
                }}
              >
                {(
                  [
                    "code",
                    "name",
                    "input_price",
                    "output_price",
                    "max_output",
                  ] as const
                ).map((field) => (
                  <label key={field} className={labelStyle}>
                    {
                      {
                        code: "API 模型 ID",
                        name: "模型显示名称",
                        input_price: "输入 $/MTok",
                        output_price: "输出 $/MTok",
                        max_output: "输出 Token 上限",
                      }[field]
                    }
                    <Input
                      className="mt-1"
                      required
                      value={newModel[field]}
                      onChange={(e) =>
                        setNewModel((prev) => ({
                          ...prev,
                          [field]: e.target.value,
                        }))
                      }
                    />
                  </label>
                ))}
                <label className={labelStyle}>
                  协议
                  <select
                    className={"mt-1 w-full " + inputStyle}
                    value={newModel.protocol}
                    onChange={(e) =>
                      setNewModel((prev) => ({
                        ...prev,
                        protocol: e.target.value,
                      }))
                    }
                  >
                    <option value="chat">Chat</option>
                    <option value="responses">Responses</option>
                    <option value="anthropic">Anthropic</option>
                    <option value="gemini">Gemini</option>
                  </select>
                </label>
                <div>
                  <Button disabled={busy} type="submit">
                    新增模型
                  </Button>
                </div>
              </form>
              <div className="space-y-2">
                {adminCatalog.map((m) => (
                  <div
                    key={m.id}
                    className="rounded-md border p-3 text-sm space-y-2"
                  >
                    <div className="font-medium">
                      {m.name}{" "}
                      <span className="text-muted-foreground">
                        ({m.code}, {m.protocol})
                      </span>
                    </div>
                    <div className="flex flex-wrap gap-2 items-end">
                      <label>
                        输入 $/MTok
                        <Input
                          type="number"
                          min="0"
                          step="0.000001"
                          className="w-28"
                          value={
                            modelEdits[m.id]?.input ?? String(m.input_price)
                          }
                          onChange={(e) =>
                            setModelEdits((prev) => ({
                              ...prev,
                              [m.id]: {
                                input: e.target.value,
                                output:
                                  prev[m.id]?.output ?? String(m.output_price),
                              },
                            }))
                          }
                        />
                      </label>
                      <label>
                        输出 $/MTok
                        <Input
                          type="number"
                          min="0"
                          step="0.000001"
                          className="w-28"
                          value={
                            modelEdits[m.id]?.output ?? String(m.output_price)
                          }
                          onChange={(e) =>
                            setModelEdits((prev) => ({
                              ...prev,
                              [m.id]: {
                                input:
                                  prev[m.id]?.input ?? String(m.input_price),
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
                              api.put("/api/shared/admin/models/" + m.id, {
                                name: m.name,
                                enabled: m.enabled,
                                max_output: m.max_output,
                                input_price: Number(
                                  modelEdits[m.id]?.input ?? m.input_price,
                                ),
                                output_price: Number(
                                  modelEdits[m.id]?.output ?? m.output_price,
                                ),
                              }),
                            "模型计费已更新",
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
                              api.put("/api/shared/admin/models/" + m.id, {
                                name: m.name,
                                enabled: !m.enabled,
                                max_output: m.max_output,
                                input_price: Number(
                                  modelEdits[m.id]?.input ?? m.input_price,
                                ),
                                output_price: Number(
                                  modelEdits[m.id]?.output ?? m.output_price,
                                ),
                              }),
                            "模型状态已更新",
                          )
                        }
                      >
                        {m.enabled ? "停用" : "启用"}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
              <h2 className="text-lg font-semibold">管理员：上游 Endpoint</h2>
              <p className="text-xs text-muted-foreground">
                Endpoint 不包含模型或模型价格。建立关联后，贡献者才能选择该
                Endpoint 支持的模型。
              </p>
              <form
                className="grid gap-3 md:grid-cols-3"
                onSubmit={(e) => {
                  e.preventDefault();
                  void mutate(
                    () => api.post("/api/shared/admin/providers", form),
                    "Endpoint 已创建，需关联模型后启用",
                  );
                }}
              >
                <label className={labelStyle}>
                  名称
                  <Input
                    required
                    className="mt-1"
                    value={form.name}
                    onChange={(e) =>
                      setForm((prev) => ({ ...prev, name: e.target.value }))
                    }
                  />
                </label>
                <label className={labelStyle}>
                  HTTPS 基址
                  <Input
                    required
                    className="mt-1"
                    value={form.base_url}
                    onChange={(e) =>
                      setForm((prev) => ({ ...prev, base_url: e.target.value }))
                    }
                  />
                </label>
                <label className={labelStyle}>
                  协议
                  <select
                    className={"mt-1 w-full " + inputStyle}
                    value={form.protocol}
                    onChange={(e) =>
                      setForm((prev) => ({ ...prev, protocol: e.target.value }))
                    }
                  >
                    <option value="chat">Chat</option>
                    <option value="responses">Responses</option>
                    <option value="anthropic">Anthropic</option>
                    <option value="gemini">Gemini</option>
                  </select>
                </label>
                <div>
                  <Button disabled={busy} type="submit">
                    新增 Endpoint
                  </Button>
                </div>
              </form>
              {adminProviders.map((p) => (
                <div key={p.id} className="space-y-2 border-t py-3 text-sm">
                  <div className="font-medium">
                    {p.name}（{p.enabled ? "已启用" : "未启用"}）
                  </div>
                  <p className="text-muted-foreground">
                    {p.base_url} · {p.protocol}
                  </p>
                  <p>
                    已绑定模型：
                    {p.models
                      .filter((m) => m.enabled_on_provider)
                      .map((m) => m.code)
                      .join("、") || "无"}
                  </p>
                  <div className="flex flex-wrap items-center gap-2">
                    <select
                      className={inputStyle}
                      value={modelToBind[p.id] ?? ""}
                      onChange={(e) =>
                        setModelToBind((prev) => ({
                          ...prev,
                          [p.id]: e.target.value,
                        }))
                      }
                    >
                      <option value="">选择关联模型</option>
                      {adminCatalog
                        .filter((m) => m.protocol === p.protocol)
                        .map((m) => (
                          <option key={m.id} value={m.id}>
                            {m.name} ({m.code})
                          </option>
                        ))}
                    </select>
                    <Button
                      disabled={busy || !modelToBind[p.id]}
                      variant="outline"
                      onClick={() =>
                        void mutate(
                          () =>
                            api.put(
                              "/api/shared/admin/providers/" + p.id + "/models",
                              {
                                model_id: Number(modelToBind[p.id]),
                                enabled: true,
                              },
                            ),
                          "模型关联已建立",
                        )
                      }
                    >
                      关联模型
                    </Button>
                    <Button
                      disabled={busy || !modelToBind[p.id]}
                      variant="outline"
                      onClick={() =>
                        void mutate(
                          () =>
                            api.put(
                              "/api/shared/admin/providers/" + p.id + "/models",
                              {
                                model_id: Number(modelToBind[p.id]),
                                enabled: false,
                              },
                            ),
                          "模型关联已停用",
                        )
                      }
                    >
                      取消关联
                    </Button>
                    <Button
                      disabled={busy}
                      variant="outline"
                      onClick={() =>
                        void mutate(
                          () =>
                            api.put("/api/shared/admin/providers/" + p.id, {
                              enabled: !p.enabled,
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
