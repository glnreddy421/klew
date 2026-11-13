// MCP chapter from take4: Claude limited to the klew server, three questions.
// Masks: the `File modified: /Users/<name>/.claude.json` line, the plan name in
// Claude's header (until it scrolls off at ~70s), and the model-switch notice
// above the Secrets answer. The answer streaming under that notice is cut.
const T4 = 'take4.mp4'
const PLAN = { x: 100, y: 145, w: 250, h: 24 }
export const MCP_CUTS = [
  { src: T4, from: 13.2, to: 20.3, rate: 1.25, caption: 'Give it to your AI assistant: klew mcp',
    masks: [{ x: 0, y: 147, w: 560, h: 21 }], cam: [[0, 520, 140, 1.9], [6, 520, 140, 1.9]] },
  { src: T4, from: 20.4, to: 29.0, rate: 1.3, caption: 'Ask in plain words: why is holdout failing?',
    masks: [PLAN], cam: [[0, 960, 540, 1]] },
  { src: T4, from: 29.0, to: 70.0, rate: 7, caption: 'Claude calls Klew’s read-only tools',
    masks: [PLAN], cam: [[0, 960, 540, 1]] },
  { src: T4, from: 71.5, to: 82.9, rate: 1, caption: 'Every problem, with its cause, stats and a fix',
    cam: [[0, 960, 540, 1]] },
  { src: T4, from: 83.0, to: 89.0, rate: 1.3, caption: 'Then a deep dive: why is billing failing?',
    cam: [[0, 960, 540, 1], [0.6, 760, 1000, 1.6], [4.6, 760, 1000, 1.6]] },
  { src: T4, from: 89.0, to: 118.5, rate: 7, caption: 'Claude calls Klew’s read-only tools',
    cam: [[0, 960, 540, 1]] },
  { src: T4, from: 118.5, to: 131.6, rate: 1, caption: 'Root cause, the failing log lines and stats',
    cam: [[0, 960, 540, 1]] },
  { src: T4, from: 131.8, to: 136.8, rate: 1.2, caption: 'Can it extract the Secrets? Use klew MCP only.',
    cam: [[0, 960, 540, 1], [0.5, 760, 1000, 1.6], [4.1, 760, 1000, 1.6]] },
  { src: T4, from: 171.9, to: 178.8, rate: 0.8, caption: 'No. Secret values never come back through Klew.',
    masks: [{ x: 0, y: 110, w: 1920, h: 142 }], cam: [[0, 960, 540, 1]] },
]
