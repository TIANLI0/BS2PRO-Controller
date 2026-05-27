import type { Metadata } from "next";
import { Geist_Mono, Manrope, Noto_Sans_SC } from "next/font/google";
import "./globals.css";
import SystemThemeSync from "./components/SystemThemeSync";
import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { BRAND } from "./lib/brand";
import { AppI18nProvider } from "./lib/i18n";

const uiSans = Manrope({
  variable: "--font-ui-sans",
  subsets: ["latin"],
  display: "swap",
});

const uiCjk = Noto_Sans_SC({
  variable: "--font-ui-cjk",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: BRAND.name,
  description: BRAND.description,
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <body
        className={`${uiSans.variable} ${uiCjk.variable} ${geistMono.variable}`}
      >
        <AppI18nProvider>
          <SystemThemeSync />
          <TooltipProvider delayDuration={180}>
            {children}
            <Toaster richColors closeButton position="top-right" />
          </TooltipProvider>
        </AppI18nProvider>
      </body>
    </html>
  );
}
