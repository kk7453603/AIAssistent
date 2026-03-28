import { ActivityFeed } from "../components/dashboard/ActivityFeed";
import { DocumentsWidget } from "../components/dashboard/DocumentsWidget";
import { MCPStatus } from "../components/dashboard/MCPStatus";
import { SchedulesWidget } from "../components/dashboard/SchedulesWidget";
import { SelfImproveWidget } from "../components/dashboard/SelfImproveWidget";
import { ToolStats } from "../components/dashboard/ToolStats";

export function DashboardPage() {
  return (
    <div className="h-full overflow-y-auto p-6">
      <div className="mx-auto max-w-5xl space-y-8">
        <section>
          <h2 className="mb-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
            Documents
          </h2>
          <DocumentsWidget />
        </section>

        <section>
          <h2 className="mb-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
            Tool Statistics
          </h2>
          <ToolStats />
        </section>

        <section>
          <h2 className="mb-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
            MCP Servers
          </h2>
          <MCPStatus />
        </section>

        <section>
          <h2 className="mb-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
            Scheduled Tasks
          </h2>
          <SchedulesWidget />
        </section>

        <section>
          <h2 className="mb-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
            Self-Improving Agent
          </h2>
          <SelfImproveWidget />
        </section>

        <section>
          <h2 className="mb-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
            Activity Feed
          </h2>
          <ActivityFeed />
        </section>
      </div>
    </div>
  );
}
