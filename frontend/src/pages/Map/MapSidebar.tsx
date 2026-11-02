import { useAuth } from '@/auth/AuthContext';
import { filterNav, PAGE_NAV } from '@/components/nav';
import { NavSections } from '@/components/NavSections';
import { SidebarCollapseButton, SidebarShell } from '@/components/SidebarShell';
import { useNavBadges } from '@/components/useNavBadges';

export type MapSidebarProps = {
  actions: {
    toggleSidebar: () => void;
  };
};

/** Map page sidebar: destinations only — map tools live in «Вид». */
export function MapSidebar({ actions }: MapSidebarProps) {
  const { toggleSidebar } = actions;
  const { isAdmin, reputationEnabled, uiAuthEnabled } = useAuth();
  const badges = useNavBadges();
  const navItems = filterNav(PAGE_NAV, {
    isAdmin,
    reputationEnabled,
    uiAuthEnabled,
  });

  return (
    <SidebarShell footer={<SidebarCollapseButton onToggle={toggleSidebar} />}>
      <NavSections items={navItems} badges={badges} />
    </SidebarShell>
  );
}
