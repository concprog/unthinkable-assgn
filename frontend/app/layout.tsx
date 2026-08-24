import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "@/components/Providers";

export const metadata: Metadata = {
  title: "Last-Mile Delivery Tracker",
  description: "Customer, agent and admin console",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className="h-full antialiased">
      <body className="min-h-full flex flex-col bg-zinc-50">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
