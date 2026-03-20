import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import OverlayScrollContainer from "./components/OverlayScrollContainer";
import SystemThemeSync from "./components/SystemThemeSync";
import { Toaster } from "@/components/ui/sonner";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "BS2PRO Controller",
  description: "BS2PRO Fan Controller Desktop App",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <SystemThemeSync />
        <OverlayScrollContainer>{children}</OverlayScrollContainer>
        <Toaster richColors closeButton position="top-right" />
      </body>
    </html>
  );
}
