"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import i18n, { type Resource } from "i18next";
import { I18nextProvider, initReactI18next } from "react-i18next";
import enUS from "../locales/en-US/translation.json";
import jaJP from "../locales/ja-JP/translation.json";
import zhCN from "../locales/zh-CN/translation.json";

export const SUPPORTED_LOCALES = ["zh-CN", "en-US", "ja-JP"] as const;
export type AppLocale = (typeof SUPPORTED_LOCALES)[number];

const DEFAULT_LOCALE: AppLocale = "zh-CN";
const LOCALE_STORAGE_KEY = "thrm.locale";

const resources: Resource = {
  "zh-CN": { translation: zhCN },
  "en-US": { translation: enUS },
  "ja-JP": { translation: jaJP },
};

function normalizeLocale(value?: string | null): AppLocale {
  if (!value) {
    return DEFAULT_LOCALE;
  }

  if (SUPPORTED_LOCALES.includes(value as AppLocale)) {
    return value as AppLocale;
  }

  const lowered = value.toLowerCase();
  if (lowered.startsWith("zh")) {
    return "zh-CN";
  }
  if (lowered.startsWith("ja")) {
    return "ja-JP";
  }
  if (lowered.startsWith("en")) {
    return "en-US";
  }

  return DEFAULT_LOCALE;
}

function readPreferredLocale(): AppLocale {
  if (typeof window === "undefined") {
    return DEFAULT_LOCALE;
  }

  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  if (stored) {
    return normalizeLocale(stored);
  }

  return normalizeLocale(window.navigator.language);
}

function syncDocumentLocale(locale: AppLocale) {
  if (typeof document === "undefined") {
    return;
  }

  document.documentElement.lang = locale;
  document.documentElement.dataset.locale = locale;
}

function syncDocumentMetadata(locale: AppLocale) {
  if (typeof document === "undefined") {
    return;
  }

  const t = i18n.getFixedT(locale);
  document.title = t("common.appName");

  const description = t("common.metaDescription");
  const descriptionMeta = document.querySelector('meta[name="description"]');
  if (descriptionMeta) {
    descriptionMeta.setAttribute("content", description);
  }
}

if (!i18n.isInitialized) {
  void i18n.use(initReactI18next).init({
    resources,
    lng: DEFAULT_LOCALE,
    fallbackLng: DEFAULT_LOCALE,
    supportedLngs: [...SUPPORTED_LOCALES],
    defaultNS: "translation",
    ns: ["translation"],
    interpolation: {
      escapeValue: false,
    },
    returnNull: false,
    initAsync: false,
  });
}

type LocaleContextValue = {
  locale: AppLocale;
  setLocale: (locale: AppLocale) => void;
  supportedLocales: readonly AppLocale[];
};

const LocaleContext = createContext<LocaleContextValue | null>(null);

export function AppI18nProvider({ children }: { children: React.ReactNode }) {
  const [locale, setLocaleState] = useState<AppLocale>(DEFAULT_LOCALE);

  useEffect(() => {
    setLocaleState(readPreferredLocale());
  }, []);

  useEffect(() => {
    syncDocumentLocale(locale);
    syncDocumentMetadata(locale);
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
    void i18n.changeLanguage(locale);
  }, [locale]);

  const value = useMemo<LocaleContextValue>(
    () => ({
      locale,
      setLocale: (nextLocale) => setLocaleState(normalizeLocale(nextLocale)),
      supportedLocales: SUPPORTED_LOCALES,
    }),
    [locale],
  );

  return (
    <I18nextProvider i18n={i18n}>
      <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>
    </I18nextProvider>
  );
}

export function useLocale() {
  const context = useContext(LocaleContext);

  if (!context) {
    throw new Error("useLocale must be used within AppI18nProvider");
  }

  return context;
}

export { i18n };
