export interface KeyboardControl {
  keys: string[][];
  action: string;
}

export function getKeyboardControls(): KeyboardControl[] {
  return [
    { keys: [["↑", "↓"], ["k", "j"]], action: "Navigate between services" },
    { keys: [["PgUp", "PgDn"]], action: "Scroll viewport" },
    { keys: [["Home", "End"]], action: "Jump to first / last service" },
    { keys: [["s"]], action: "Stop or start the selected service" },
    { keys: [["r"]], action: "Restart the selected service" },
    { keys: [["/"]], action: "Filter services by name" },
    { keys: [["Esc"]], action: "Clear filter" },
    { keys: [["q"]], action: "Quit and stop all services" },
  ];
}
