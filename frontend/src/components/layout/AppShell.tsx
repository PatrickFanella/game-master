import type { ReactNode } from 'react';

import { UserMenu } from './UserMenu';

interface AppShellProps {
  readonly title: string;
  readonly description: string;
  readonly actions?: ReactNode;
  readonly children: ReactNode;
}

export function AppShell({ title, description, actions, children }: AppShellProps) {
  return (
    <main className="min-h-screen bg-obsidian px-6 py-16 text-champagne">
      <div className="deco-corners deco-pattern mx-auto flex w-full max-w-5xl flex-col gap-8 border-2 border-gold/20 bg-charcoal p-8">
        <header className="flex flex-col gap-6 border-b-2 border-gold/20 pb-6">
          <div className="flex items-center justify-between">
            <p className="font-heading text-sm font-semibold uppercase tracking-[0.32em] text-gold">Game Master</p>
            <UserMenu />
          </div>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="space-y-2">
              <h1 className="font-heading text-3xl font-semibold uppercase tracking-[0.12em] text-champagne sm:text-4xl">{title}</h1>
              <p className="max-w-2xl text-sm leading-7 text-champagne/70 sm:text-base">{description}</p>
            </div>
            {actions ? <div className="flex shrink-0 items-center gap-3">{actions}</div> : null}
          </div>
        </header>
        <section>{children}</section>
      </div>
    </main>
  );
}
