'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useQueryClient } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api';
import type { Role } from '@/lib/queries';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

type NavItem = { href: string; label: string; roles: Role[] };

const NAV_ITEMS: NavItem[] = [
  { href: '/operators', label: 'ผู้ประกอบการ', roles: ['super_admin'] },
  {
    href: '/piers',
    label: 'ท่าเรือ',
    roles: ['super_admin', 'pier_admin', 'staff'],
  },
  {
    href: '/routes',
    label: 'เส้นทาง',
    roles: ['super_admin', 'pier_admin', 'staff'],
  },
  {
    href: '/boats',
    label: 'เรือ',
    roles: ['super_admin', 'pier_admin', 'staff'],
  },
  { href: '/staff', label: 'ผู้ใช้งาน', roles: ['super_admin'] },
];

export function AdminShell({
  role,
  children,
}: {
  role: Role;
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const items = NAV_ITEMS.filter((item) => item.roles.includes(role));

  async function logout() {
    await apiFetch('/api/v1/auth/logout', { method: 'POST' }).catch(() => {});
    queryClient.clear();
    router.replace('/login');
  }

  return (
    <div className="flex min-h-svh flex-col md:flex-row">
      <aside className="hidden shrink-0 flex-col gap-1 border-r bg-sidebar p-4 md:flex md:w-56">
        <NavLinks items={items} pathname={pathname} />
        <div className="mt-auto pt-4">
          <Button variant="outline" className="w-full" onClick={logout}>
            ออกจากระบบ
          </Button>
        </div>
      </aside>
      <header className="flex items-center justify-between gap-2 overflow-x-auto border-b bg-sidebar p-2 md:hidden">
        <nav className="flex gap-1">
          <NavLinks items={items} pathname={pathname} />
        </nav>
        <Button variant="outline" size="sm" onClick={logout}>
          ออกจากระบบ
        </Button>
      </header>
      <main className="flex-1 overflow-x-hidden p-4 md:p-6">{children}</main>
    </div>
  );
}

function NavLinks({ items, pathname }: { items: NavItem[]; pathname: string }) {
  return (
    <>
      {items.map((item) => {
        const active = pathname.startsWith(item.href);
        return (
          <Link
            key={item.href}
            href={item.href}
            className={cn(
              'rounded-lg px-3 py-2 text-sm font-medium whitespace-nowrap transition-colors',
              active
                ? 'bg-primary text-primary-foreground'
                : 'text-sidebar-foreground hover:bg-sidebar-accent',
            )}
          >
            {item.label}
          </Link>
        );
      })}
    </>
  );
}
