import type { ReactNode } from "react";
import { ConnectionStatus } from "./ConnectionStatus";

interface LayoutProps {
  children: ReactNode;
}

export function Layout({ children }: LayoutProps) {
  return (
    <div className="flex h-screen flex-col">
      {/* Header */}
      <header className="flex items-center justify-between border-b border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 px-4 py-3">
        <h1 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
          Personal AI Assistant
        </h1>
        <ConnectionStatus />
      </header>

      <div className="flex flex-1 overflow-hidden">
        {/* Sidebar */}
        <aside className="w-56 shrink-0 border-r border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900 p-4">
          <nav className="space-y-1 text-sm text-gray-500 dark:text-gray-400">
            <p className="text-xs uppercase tracking-wider text-gray-500">
              Navigation
            </p>
          </nav>
        </aside>

        {/* Main content */}
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
