export interface Demo {
  slug: string;
  title: string;
  description: string;
  before: string;
  input: string;
  steps: string[];
  benefit: string;
  agent: boolean;
  feature: { label: string; href: string };
}

export function getDemos(base: string): Demo[] {
  return [
    {
      slug: "agent-hot-reload",
      title: "Agent fixes, fuku reloads",
      description: "The agent fixes a bug in the api code. fuku sees the change and restarts api without a command.",
      before: "fuku runs six services. The api service has a watch rule. It starts, then crashes after four seconds.",
      input: "api keeps crashing, find the bug and fix it",
      steps: [
        "The agent reads the api logs through fuku and finds an index out of range.",
        "The agent changes one line in main.go.",
        "fuku sees the change and restarts api.",
        "The agent checks through fuku that api stays up.",
      ],
      benefit: "The agent changes only the code. fuku restarts the service and the agent reads the result from fuku.",
      agent: true,
      feature: { label: "Hot-Reload", href: `${base}features/hot-reload/` },
    },
    {
      slug: "agent-fix",
      title: "Agent fixes a failed service",
      description: "The api service does not start. The agent finds the cause in the logs, fixes it and restarts only api.",
      before: "fuku runs six services. The api service stops at startup. fuku tries it again, then marks it failed.",
      input: "api is down. Find out why, fix it and bring it back up",
      steps: [
        "The agent reads the status and the api logs through fuku.",
        "The log shows that BOOKS_DB_URL is not set.",
        "The agent creates .env from .env.example.",
        "The agent restarts api through fuku. The other five services continue to run.",
      ],
      benefit: "The agent does not search for processes, ports or log files. fuku gives it the state and the logs.",
      agent: true,
      feature: { label: "AI Agents", href: `${base}features/agents/` },
    },
    {
      slug: "agent-doctor",
      title: "Agent fixes a config typo",
      description: "A typo in fuku.yaml stops fuku. The agent finds the typo with the doctor, fixes it and starts fuku.",
      before: "fuku.yaml has the readiness type htttp. fuku run stops with a config error.",
      input: "fuku won't start. Find out why and fix it",
      steps: [
        "The agent runs the doctor through fuku. The doctor works even when the config is not valid.",
        "The agent changes htttp to http in fuku.yaml.",
        "The agent starts the default profile.",
        "On the left, fuku doctor --summary shows the config as valid and fuku as running.",
      ],
      benefit: "The doctor tells the agent which line is wrong. The agent does not guess from the error text.",
      agent: true,
      feature: { label: "AI Agents", href: `${base}features/agents/` },
    },
    {
      slug: "agent-init",
      title: "Agent writes the fuku.yaml",
      description: "The project has services but no fuku.yaml. The agent reads the services and writes the config.",
      before: "The bookstore example has six services and no fuku.yaml.",
      input: "Write a fuku.yaml for the services in examples/bookstore",
      steps: [
        "The agent reads the code of each service.",
        "The agent writes fuku.yaml with tiers, readiness checks, watch rules and profiles.",
        "On the left, fuku run starts all six services from the new file.",
      ],
      benefit: "A new project gets a working config from one sentence.",
      agent: true,
      feature: { label: "AI Agents", href: `${base}features/agents/` },
    },
    {
      slug: "agent-profiles",
      title: "Agent switches profiles",
      description: "The agent starts one profile in the background. Then it stops that profile and starts a different one.",
      before: "No service runs.",
      input: "Start the core profile\nSwitch to minimal and show me what is running",
      steps: [
        "The agent starts the core profile in the background.",
        "The agent stops fuku and starts the minimal profile.",
        "The agent shows a table of the services that run.",
      ],
      benefit: "You ask for a set of services. The agent knows the profiles and the fuku commands.",
      agent: true,
      feature: { label: "AI Agents", href: `${base}features/agents/` },
    },
    {
      slug: "startup",
      title: "Tiered startup",
      description: "Six services start in three tiers. Each tier starts when the tier before it is ready.",
      before: "No service runs.",
      input: "fuku run",
      steps: [
        "The foundation tier starts first, then platform, then edge.",
        "A log, HTTP or TCP check tells fuku when a service is ready.",
        "The info panel shows the readiness check of api.",
        "The q key stops the services in the opposite order.",
      ],
      benefit: "A service starts only after the services that it needs are ready.",
      agent: false,
      feature: { label: "Orchestration", href: `${base}features/orchestration/` },
    },
    {
      slug: "tui",
      title: "TUI controls",
      description: "Filter the list, restart a service, stop it and start it, and open its info panel.",
      before: "fuku runs six services.",
      input: "fuku run",
      steps: [
        "The / key filters the list. The Esc key clears the filter.",
        "The r key restarts api. The s key stops api and starts it again.",
        "The Enter key opens the info panel of auth.",
        "The Tab key shows the config, env and health tabs.",
      ],
      benefit: "You control each service from the keyboard. You do not open a second terminal.",
      agent: false,
      feature: { label: "Interactive TUI", href: `${base}features/tui/` },
    },
    {
      slug: "logs",
      title: "Log streaming",
      description: "fuku logs shows the output of the services that you select, in a second terminal.",
      before: "fuku runs six services on the left.",
      input: "fuku logs api storage",
      steps: [
        "The stream on the right shows only the api and storage lines.",
        "The TUI on the left restarts storage.",
        "The stream shows storage stop and start again.",
      ],
      benefit: "You read the logs of two services and ignore the other four.",
      agent: false,
      feature: { label: "Log Streaming", href: `${base}features/log-streaming/` },
    },
    {
      slug: "preflight",
      title: "Preflight cleanup",
      description: "An old run left api and storage running. fuku stops them before it starts the services.",
      before: "api and storage from an old run still run. api still answers on port 8080.",
      input: "fuku run",
      steps: [
        "fuku finds the processes that run in the service directories.",
        "fuku stops them. The header shows \"preflight: stopping api\".",
        "All six services start on free ports.",
      ],
      benefit: "A crashed run does not block the next run with a port that is in use.",
      agent: false,
      feature: { label: "Lifecycle", href: `${base}features/lifecycle/` },
    },
    {
      slug: "detached",
      title: "Detached run",
      description: "Start a profile in the background and keep your terminal. Read the logs, then stop the profile.",
      before: "No service runs.",
      input: "fuku run core -d",
      steps: [
        "fuku shows the progress and returns when all services run.",
        "The api service answers on port 8080.",
        "fuku logs --no-follow shows the last api lines.",
        "fuku stop stops the profile.",
      ],
      benefit: "The services run in the background. Your terminal stays free for other work.",
      agent: false,
      feature: { label: "Lifecycle", href: `${base}features/lifecycle/` },
    },
  ];
}
