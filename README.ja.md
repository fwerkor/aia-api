# AIA API

[简体中文](README.zh_CN.md) · [繁體中文](README.zh_TW.md) · [English](README.md) · [Français](README.fr.md) · **日本語**

AIA API は南京大学の学生 AI コミュニティ向けの、セルフホスト型モデル API ゲートウェイおよび**利用者提供 API キーの共有プール**です。利用者は **AIA Forum OIDC** でログインし、共通クレジットを利用できます。各プロバイダーの規約で許可される場合に限り、開発者 API キーを提供できます。

このリポジトリは [New API](https://github.com/QuantumNous/new-api) を元にした AIA 独自派生版であり、上流の公式 New API ではありません。[AGPL-3.0-or-later](LICENSE)、[NOTICE](NOTICE)、[サードパーティライセンス](THIRD-PARTY-LICENSES.md)を参照してください。

## 主な機能

- **SSO ログイン**：AIA Forum OIDC を使用。通常のパスワード登録・ログインは無効です。
- **OpenAI 互換 API**：プラットフォーム発行の Token で `/v1` を利用します。モデルの使用可否は検証済みの有効な上流チャネルに依存します。
- **API キー共有**：開発者 API キーの登録、モデル別検証、暗号化保存、日次利用上限。成功した利用の精算後に貢献者へクレジットを付与します。
- **独立したモデル一覧と上流管理**：参照価格は人民元建てです。
- **引換コード**：クレジットの引換は可能ですが、現金チャージやオンライン決済、商用サブスクリプション購入は提供しません。一時的なレート制限でキーが永久停止されることもありません。

## 利用方法

`https://api.nju-aia.com`（または管理者指定のサイト）へアクセスし、AIA Forum でログインしてください。API Token を発行し、OpenAI 互換クライアントの Base URL を `https://api.nju-aia.com/v1` に設定します。利用可能なモデル ID を指定してください。`/wallet` でクレジットや引換コード、`/shared-pool` で API キーの提供を管理します。ブラウザー Cookie、IDE の OAuth トークン、定額契約の認証情報を登録しないでください。

## セルフホスト

```bash
git clone https://github.com/fwerkor/aia-api.git
cd aia-api
openssl rand -hex 32  # 値を安全に保管し AIA_SHARED_SECRET に設定
docker compose up -d
```

イメージは `ghcr.io/fwerkor/aia-api:latest` です。Compose にあるサンプルの DB・Redis パスワードは必ず変更し、HTTPS、Forum OIDC（コールバック `https://YOUR_DOMAIN/oauth/oidc`）、DB バックアップを設定してください。暗号化キーは変更せず安全に保管します。`AIA_SHARED_SECRET` がない場合、共有プールは使用できません。

## ドキュメント・サポート

[目次](docs/README.md) · [ユーザーガイド](docs/usage.md) · [導入ガイド](docs/deployment.md) · [管理者ガイド](docs/admin.md) · [API](docs/api.md) · [共有プール設計](AIA-SHARED-POOL.md)

不具合は [AIA API Issues](https://github.com/fwerkor/aia-api/issues)、セキュリティの問題は[非公開で報告](https://github.com/fwerkor/aia-api/security/advisories/new)してください。
