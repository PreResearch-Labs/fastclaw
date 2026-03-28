import { AuthGuard } from "@/components/auth-guard";
import { SidebarLayout } from "@/components/sidebar";

export default function OverviewLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <AuthGuard>
      <SidebarLayout>{children}</SidebarLayout>
    </AuthGuard>
  );
}