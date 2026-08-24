import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";
import { Providers } from "@/components/Providers";
import { LogoutButton } from "@/components/LogoutButton";

export const metadata: Metadata = {
  title: "Last-Mile Delivery Tracker",
  description: "Customer, agent and admin console",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full flex flex-col bg-zinc-50 text-zinc-900">
        <Providers>
          <header className="flex items-center justify-between border-b border-zinc-200 bg-white px-6 py-2.5">
            <Link href="/" className="text-sm font-semibold tracking-tight">
              Last-Mile Tracker
            </Link>
            <LogoutButton />
          </header>
          {children}
        </Providers>
      </body>
    </html>
  );
}
