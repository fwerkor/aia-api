# AIA API

[简体中文](README.zh_CN.md) · [繁體中文](README.zh_TW.md) · [English](README.md) · **Français** · [日本語](README.ja.md)

AIA API est une passerelle de modèles d'IA auto-hébergée et une **réserve de clés API contribuées** pour la communauté étudiante AIA de l'Université de Nankin. Les membres se connectent via **AIA Forum OIDC**, utilisent des crédits communs et peuvent proposer des clés API de développeur lorsque les conditions du fournisseur l'autorisent.

Ce dépôt est une adaptation de [New API](https://github.com/QuantumNous/new-api), **pas** le projet officiel en amont. Licence [AGPL-3.0-or-later](LICENSE) ; voir [NOTICE](NOTICE) et [les licences tierces](THIRD-PARTY-LICENSES.md).

## Fonctionnalités

- **Connexion unique** via AIA Forum OIDC, sans inscription ni connexion par mot de passe native.
- **API compatible OpenAI** : accès à `/v1` avec un jeton délivré par la plateforme ; seuls les modèles dont les canaux sont actifs et vérifiés peuvent répondre.
- **Partage de clés** : contribution volontaire de clés API de développeur, vérification des modèles, stockage chiffré et plafond journalier. Les contributeurs reçoivent des crédits après règlement effectif des appels.
- **Catalogue indépendant** des modèles et fournisseurs, tarifs de référence en CNY et comptabilité des crédits.
- **Codes de crédit** disponibles ; pas de rechargement en argent réel, de paiement en ligne ou d'achat d'abonnement commercial. Une erreur de quota temporaire n'entraîne pas le bannissement permanent d'une clé.

## Utilisation

Connectez-vous à `https://api.nju-aia.com` (ou à l'adresse de votre administrateur) avec le compte du forum. Créez un jeton API et configurez un client compatible OpenAI avec `https://api.nju-aia.com/v1` comme URL de base, puis utilisez un ID de modèle disponible. Le portefeuille et l'échange de codes sont accessibles sur `/wallet` ; les contributions sur `/shared-pool`. N'envoyez jamais de cookies de navigateur ni de jetons OAuth d'abonnement.

## Déploiement

```bash
git clone https://github.com/fwerkor/aia-api.git
cd aia-api
openssl rand -hex 32  # à stocker de manière privée dans AIA_SHARED_SECRET
docker compose up -d
```

L'image utilisée est `ghcr.io/fwerkor/aia-api:latest`. **Changez les mots de passe d'exemple** de PostgreSQL et Redis, configurez HTTPS et le fournisseur OIDC du forum (URI de retour : `https://VOTRE_DOMAINE/oauth/oidc`), puis sauvegardez la base et la clé de chiffrement persistante. Sans `AIA_SHARED_SECRET`, le partage des clés est indisponible.

## Documentation et support

[Sommaire](docs/README.md) · [Guide utilisateur](docs/usage.md) · [Déploiement](docs/deployment.md) · [Administration](docs/admin.md) · [API](docs/api.md) · [Conception du partage](AIA-SHARED-POOL.md)

Signalez les problèmes sur [AIA API Issues](https://github.com/fwerkor/aia-api/issues) et les vulnérabilités [en privé](https://github.com/fwerkor/aia-api/security/advisories/new).
