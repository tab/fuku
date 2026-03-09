export interface NavItem {
  label: string;
  href: string;
  indent?: boolean;
}

export function getDocsNav(base: string): NavItem[] {
  return [
    { label: "Overview", href: `${base}docs/` },
    { label: "Getting Started", href: `${base}docs/getting-started/` },
    { label: "Configuration", href: `${base}docs/configuration/` },
    { label: "CLI Commands", href: `${base}docs/cli/` },
    { label: "Examples", href: `${base}docs/examples/` },
    { label: "Troubleshooting", href: `${base}docs/troubleshooting/` },
    { label: "Privacy", href: `${base}docs/privacy/` },
  ];
}

export function getFeaturesNav(base: string): NavItem[] {
  return [
    { label: "Overview", href: `${base}features/` },
    { label: "Interactive TUI", href: `${base}features/tui/` },
    { label: "Orchestration", href: `${base}features/orchestration/` },
    { label: "Readiness Checks", href: `${base}features/readiness/` },
    { label: "Hot-Reload", href: `${base}features/hot-reload/` },
    { label: "Log Streaming", href: `${base}features/log-streaming/` },
    { label: "Lifecycle", href: `${base}features/lifecycle/` },
  ];
}
