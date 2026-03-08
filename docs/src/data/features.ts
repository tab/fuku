export interface Feature {
  title: string;
  description: string;
  icon: string;
  href: string;
}

export function getFeatures(base: string): Feature[] {
  return [
    {
      title: "Interactive TUI",
      description:
        "Real-time service dashboard with CPU/memory monitoring, log viewing, and keyboard-driven controls.",
      icon: "terminal",
      href: `${base}features/tui/`,
    },
    {
      title: "Orchestration",
      description:
        "Tier-based startup ordering ensures dependencies come up before the services that need them.",
      icon: "layers",
      href: `${base}features/orchestration/`,
    },
    {
      title: "Readiness Checks",
      description:
        "HTTP, TCP, and log-based health checks confirm services are truly ready before proceeding.",
      icon: "check",
      href: `${base}features/readiness/`,
    },
    {
      title: "Hot-Reload",
      description:
        "File watcher with debouncing automatically restarts services when source files change.",
      icon: "refresh",
      href: `${base}features/hot-reload/`,
    },
    {
      title: "Log Streaming",
      description:
        "Unix socket-based log streaming with service filtering and formatted output.",
      icon: "scroll",
      href: `${base}features/log-streaming/`,
    },
    {
      title: "Lifecycle",
      description:
        "Graceful shutdown with SIGTERM/SIGKILL handling, reverse-order teardown, and pre-flight cleanup.",
      icon: "shield",
      href: `${base}features/lifecycle/`,
    },
  ];
}
