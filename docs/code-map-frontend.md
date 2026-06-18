# KleinAI — Frontend Code Map

## apps/admin/src/ (Admin SPA)
```
pages/auth/LoginPage.tsx      pages/dashboard/DashboardPage.tsx
pages/accounts/TokenAccountsPage.tsx
pages/users/UsersPage.tsx     pages/proxies/ProxiesPage.tsx
pages/promo/CDKPage.tsx       pages/promo/PromoPage.tsx
pages/billing/BillingPage.tsx pages/logs/LogsPage.tsx
pages/system/ConfigPage.tsx   pages/system/ModelPricesPage.tsx
pages/system/BillingSettingsPage.tsx  pages/system/RechargePackagesPage.tsx
layouts/AdminLayout.tsx       routes/RequireAuth.tsx
lib/api.ts  lib/services.ts  lib/types.ts  lib/format.ts
stores/auth.ts  stores/toast.ts  components/Logo.tsx  components/Toaster.tsx
```

## apps/user/src/ (User SPA)
```
pages/auth/LoginPage.tsx      pages/auth/RegisterPage.tsx
pages/create/CreateImagePage.tsx  pages/create/CreateVideoPage.tsx
pages/create/CreateStudioPage.tsx  pages/create/HistoryPage.tsx
pages/billing/BillingPage.tsx  pages/keys/KeysPage.tsx  pages/keys/DocsPage.tsx
pages/invite/InvitePage.tsx    pages/settings/SettingsPage.tsx
layouts/AppLayout.tsx  layouts/AuthLayout.tsx
lib/api.ts  lib/services.ts  lib/types.ts  lib/format.ts
stores/auth.ts  stores/loginGate.ts  stores/toast.ts
components/LoginGate.tsx  components/LoadingScreen.tsx  components/Logo.tsx  components/Toaster.tsx
```

## packages/theme/ (Shared)
```
src/index.ts          src/tailwind.preset.ts
```