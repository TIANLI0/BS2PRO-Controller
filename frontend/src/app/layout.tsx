import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import OverlayScrollContainer from "./components/OverlayScrollContainer";
import SystemThemeSync from "./components/SystemThemeSync";

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
  description: "BS2PRO 压风控制器桌面端",
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
      </body>
    </html>
  );
}
