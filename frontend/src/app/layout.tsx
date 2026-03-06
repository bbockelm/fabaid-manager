import type { Metadata } from 'next';
import { Inter } from 'next/font/google';
import './globals.css';
import { Providers } from './providers';
import { AppShell } from '@/components/AppShell';

const inter = Inter({ subsets: ['latin'] });

export const metadata: Metadata = {
  title: 'FabAID — Project Tracker',
  description: 'Track project budgets, personnel, subawards, and invoices',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body className={inter.className}>
        <Providers>
          <AppShell>{children}</AppShell>
        </Providers>
      </body>
    </html>
  );
}
