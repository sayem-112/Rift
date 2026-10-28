import type { ReactNode } from "react";

/*
 * Illustrated cover art for the sample library and the landing page.
 *
 * In the real app a cover comes from the book itself (the EPUB's cover image,
 * or a PDF's first page); the generated typographic cover is the fallback.
 * Until extraction exists, these stand in for those images. They are original
 * drawings for public-domain titles, in the spirit of classic paperback
 * editions, not copies of any published cover.
 *
 * Every edition is drawn on a 200 x 300 canvas and scales as one image, so the
 * same art serves 24px thumbnails and 220px covers.
 */

export interface Edition {
  /** Two colors that sum up the cover; used for the ambient light behind it. */
  glow: [string, string];
  art: ReactNode;
}

const SERIF = { fontFamily: '"Literata Variable", Georgia, serif' } as const;
const SANS = { fontFamily: "var(--font-display)" } as const;
const UI = { fontFamily: "var(--font-ui)" } as const;

/* Leaves around an arc: Meditations' laurel. */
function laurel(cx: number, cy: number, r: number, side: 1 | -1, color: string) {
  const leaves: ReactNode[] = [];
  for (let i = 0; i < 9; i++) {
    const a = ((100 - i * 17) * Math.PI) / 180;
    const x = cx + side * r * Math.cos(a);
    const y = cy + r * Math.sin(a);
    const tangent = (Math.atan2(Math.cos(a), -Math.sin(a)) * 180) / Math.PI;
    const rot = side === 1 ? tangent + 28 : 180 - tangent - 28;
    leaves.push(<ellipse key={i} cx={x} cy={y} rx={10 - i * 0.5} ry={3.8 - i * 0.15} transform={`rotate(${rot} ${x} ${y})`} fill={color} />);
  }
  return leaves;
}

/* A row of sine waves for the sea. */
function wave(y: number, amp: number, len: number, phase = 0) {
  let d = `M -20 ${y}`;
  for (let x = -20; x <= 220; x += len / 2) {
    const up = Math.round((x + phase) / (len / 2)) % 2 === 0;
    d += ` q ${len / 4} ${up ? -amp : amp} ${len / 2} 0`;
  }
  return d;
}

export const EDITIONS: Record<string, Edition> = {
  meditations: {
    glow: ["#2b3a7a", "#e6c27a"],
    art: (
      <>
        <rect width="200" height="300" fill="#1b2346" />
        <circle cx="100" cy="128" r="66" fill="#222c58" />
        <text x="100" y="40" textAnchor="middle" fontSize="7.5" fontStyle="italic" fill="#c9cde8" opacity="0.75" style={SERIF}>
          translated by George Long
        </text>
        {laurel(100, 128, 48, 1, "#e6c27a")}
        {laurel(100, 128, 48, -1, "#e6c27a")}
        <text x="100" y="140" textAnchor="middle" fontSize="30" fill="#e6c27a" style={SERIF}>
          M
        </text>
        <text x="100" y="236" textAnchor="middle" fontSize="18" letterSpacing="3.5" fill="#f0dcae" style={SERIF}>
          MEDITATIONS
        </text>
        <rect x="88" y="248" width="24" height="1" fill="#e6c27a" opacity="0.7" />
        <text x="100" y="266" textAnchor="middle" fontSize="7.5" letterSpacing="2.6" fill="#c9cde8" style={SANS}>
          MARCUS AURELIUS
        </text>
      </>
    ),
  },

  sicp: {
    glow: ["#5b3c88", "#f3d36b"],
    art: (
      <>
        <rect width="200" height="300" fill="#4f3479" />
        <circle cx="128" cy="196" r="78" fill="#5d3f8c" />
        <text x="122" y="268" textAnchor="middle" fontSize="190" fontStyle="italic" fill="#f3d36b" style={SERIF}>
          λ
        </text>
        <g fontSize="15.5" fontWeight="700" fill="#fff" letterSpacing="-0.4" style={SANS}>
          <text x="16" y="36">Structure and</text>
          <text x="16" y="54">Interpretation</text>
          <text x="16" y="72">of Computer</text>
          <text x="16" y="90">Programs</text>
        </g>
        <text x="16" y="282" fontSize="8" fill="#e4d7f7" style={UI}>
          Abelson &amp; Sussman
        </text>
      </>
    ),
  },

  odyssey: {
    glow: ["#0f3b52", "#e8835a"],
    art: (
      <>
        <rect width="200" height="300" fill="#0d3349" />
        <circle cx="138" cy="122" r="28" fill="#e8835a" />
        <rect y="150" width="200" height="150" fill="#0a2a3d" />
        <path d="M 74 168 L 74 132 L 98 164 Z" fill="#f2e3c9" />
        <path d="M 62 168 L 104 168 L 98 176 L 68 176 Z" fill="#c9673f" />
        {[0, 1, 2, 3, 4].map((i) => (
          <path key={i} d={wave(186 + i * 20, 5, 34, i * 9)} fill="none" stroke="#2f7593" strokeWidth="2.4" opacity={1 - i * 0.12} />
        ))}
        <text x="100" y="40" textAnchor="middle" fontSize="8" letterSpacing="3.4" fill="#9cc7d9" style={SANS}>
          HOMER
        </text>
        <text x="100" y="74" textAnchor="middle" fontSize="27" fontStyle="italic" fill="#f2e3c9" style={SERIF}>
          The Odyssey
        </text>
      </>
    ),
  },

  frankenstein: {
    glow: ["#1c3a2c", "#b8e0b0"],
    art: (
      <>
        <rect width="200" height="300" fill="#121e19" />
        <circle cx="100" cy="122" r="62" fill="#b8e0b0" opacity="0.92" />
        <path d="M 100 48 L 100 196" stroke="#121e19" strokeWidth="2.4" />
        {[70, 86, 102, 118, 134, 150, 166].map((y) => (
          <path key={y} d={`M 92 ${y} L 108 ${y + 4}`} stroke="#121e19" strokeWidth="2.4" strokeLinecap="round" />
        ))}
        <text x="100" y="246" textAnchor="middle" fontSize="20" fontWeight="700" letterSpacing="0.6" fill="#d6f0cf" style={SANS}>
          FRANKENSTEIN
        </text>
        <text x="100" y="266" textAnchor="middle" fontSize="9.5" fontStyle="italic" fill="#9fbf98" style={SERIF}>
          Mary Shelley
        </text>
      </>
    ),
  },

  moby: {
    glow: ["#1f3557", "#efe6d2"],
    art: (
      <>
        <rect width="200" height="300" fill="#efe6d2" />
        <path
          d="M 94 214 C 96 192 95 174 92 162 C 80 154 60 152 46 132 C 68 136 88 142 100 152 C 112 142 132 136 154 132 C 140 152 120 154 108 162 C 105 174 104 192 106 214 Z"
          fill="#1f3557"
        />
        <rect y="206" width="200" height="94" fill="#efe6d2" />
        <path d={wave(208, 3.5, 22)} fill="none" stroke="#1f3557" strokeWidth="2" />
        <path d={wave(220, 3, 26, 6)} fill="none" stroke="#1f3557" strokeWidth="1.4" opacity="0.5" />
        <text x="100" y="58" textAnchor="middle" fontSize="26" fontWeight="600" fill="#1f3557" style={SERIF}>
          Moby-Dick
        </text>
        <text x="100" y="76" textAnchor="middle" fontSize="9" fontStyle="italic" fill="#1f3557" opacity="0.75" style={SERIF}>
          or, The Whale
        </text>
        <text x="100" y="270" textAnchor="middle" fontSize="7.5" letterSpacing="2.6" fill="#1f3557" style={SANS}>
          HERMAN MELVILLE
        </text>
      </>
    ),
  },

  "art-of-war": {
    glow: ["#b5332e", "#f4e9d8"],
    art: (
      <>
        <rect width="200" height="300" fill="#b3352f" />
        <path
          d="M 132 66 A 54 54 0 1 1 74 74"
          fill="none"
          stroke="#1a1414"
          strokeWidth="15"
          strokeLinecap="round"
          transform="rotate(8 100 120)"
        />
        <circle cx="146" cy="58" r="3" fill="#1a1414" />
        <g fontSize="22" fontWeight="700" fill="#f4e9d8" letterSpacing="0.5" style={SANS}>
          <text x="18" y="232">THE ART</text>
          <text x="18" y="256">OF WAR</text>
        </g>
        <text x="18" y="278" fontSize="9.5" fontStyle="italic" fill="#f4e9d8" opacity="0.8" style={SERIF}>
          Sun Tzu
        </text>
      </>
    ),
  },

  "think-python": {
    glow: ["#1f6f6b", "#f4c95d"],
    art: (
      <>
        <rect width="200" height="300" fill="#1c625f" />
        <path
          d="M 40 236 C 40 196 160 214 160 174 C 160 136 50 156 50 120 C 50 96 90 92 112 100"
          fill="none"
          stroke="#f4c95d"
          strokeWidth="17"
          strokeLinecap="round"
        />
        <circle cx="118" cy="96" r="2.6" fill="#1c625f" />
        <text x="16" y="42" fontSize="25" fontWeight="700" fill="#fff" letterSpacing="-0.6" style={SANS}>
          Think Python
        </text>
        <text x="16" y="58" fontSize="7.5" fill="#d3ecea" style={UI}>
          How to Think Like a Computer Scientist
        </text>
        <text x="16" y="282" fontSize="8" fill="#d3ecea" style={UI}>
          Allen B. Downey
        </text>
      </>
    ),
  },

  origin: {
    glow: ["#2e4a36", "#e9e4d0"],
    art: (
      <>
        <rect width="200" height="300" fill="#e9e4d0" />
        <g fill="none" stroke="#2e4a36" strokeWidth="1.8" strokeLinecap="round">
          <path d="M 100 236 L 100 202 L 72 176 L 60 150 M 72 176 L 84 146 M 100 202 L 126 182 L 120 156 M 126 182 L 148 166 L 152 140 M 148 166 L 166 156 M 60 150 L 50 130 M 60 150 L 66 128 M 84 146 L 92 128 M 120 156 L 112 136 M 120 156 L 130 134" />
        </g>
        <g fill="#2e4a36">
          {[
            [50, 130],
            [66, 128],
            [92, 128],
            [112, 136],
            [130, 134],
            [152, 140],
            [166, 156],
          ].map(([x, y]) => (
            <circle key={`${x}-${y}`} cx={x} cy={y} r="3" />
          ))}
        </g>
        <text x="122" y="226" fontSize="10" fontStyle="italic" fill="#2e4a36" opacity="0.7" style={SERIF}>
          I think
        </text>
        <g textAnchor="middle" fontSize="17" fill="#2e4a36" style={SERIF}>
          <text x="100" y="48">On the Origin</text>
          <text x="100" y="68">of Species</text>
        </g>
        <text x="100" y="274" textAnchor="middle" fontSize="7.5" letterSpacing="2.6" fill="#2e4a36" style={SANS}>
          CHARLES DARWIN
        </text>
      </>
    ),
  },

  middlemarch: {
    glow: ["#7a3b4b", "#f0c8b8"],
    art: (
      <>
        <rect width="200" height="300" fill="#6f3646" />
        <path
          d="M 0 300 L 0 196 L 18 180 L 36 196 L 36 172 L 58 152 L 80 172 L 80 188 L 98 170 L 116 188 L 116 160 L 140 138 L 164 160 L 164 182 L 182 166 L 200 182 L 200 300 Z"
          fill="#f0c8b8"
        />
        <g fill="#6f3646">
          {[
            [12, 204],
            [24, 204],
            [48, 182],
            [62, 182],
            [48, 204],
            [62, 204],
            [90, 198],
            [104, 198],
            [128, 170],
            [146, 170],
            [128, 194],
            [146, 194],
            [174, 194],
            [188, 194],
          ].map(([x, y]) => (
            <rect key={`${x}-${y}`} x={x} y={y} width="7" height="10" rx="1" />
          ))}
        </g>
        <text x="100" y="78" textAnchor="middle" fontSize="27" fontStyle="italic" fill="#f6dccf" style={SERIF}>
          Middlemarch
        </text>
        <text x="100" y="98" textAnchor="middle" fontSize="7.5" letterSpacing="2.6" fill="#f0c8b8" opacity="0.85" style={SANS}>
          GEORGE ELIOT
        </text>
      </>
    ),
  },

  walden: {
    glow: ["#26402f", "#8fc1b5"],
    art: (
      <>
        <rect width="200" height="300" fill="#223a2b" />
        <g fill="#14261b">
          {[24, 52, 84, 118, 150, 178].map((x, i) => (
            <path key={x} d={`M ${x} ${130 - (i % 2) * 16} L ${x + 18} 196 L ${x - 18} 196 Z`} />
          ))}
        </g>
        <ellipse cx="100" cy="214" rx="86" ry="20" fill="#8fc1b5" />
        <g fill="#6fa395" opacity="0.8">
          {[52, 84, 118, 150].map((x) => (
            <path key={x} d={`M ${x - 9} 202 L ${x + 9} 202 L ${x} 226 Z`} />
          ))}
        </g>
        <text x="100" y="66" textAnchor="middle" fontSize="24" fontWeight="600" letterSpacing="6" fill="#efe7d2" style={SANS}>
          WALDEN
        </text>
        <text x="100" y="84" textAnchor="middle" fontSize="9" fontStyle="italic" fill="#c7d6c4" style={SERIF}>
          or, Life in the Woods
        </text>
        <text x="100" y="270" textAnchor="middle" fontSize="7.5" letterSpacing="2.2" fill="#c7d6c4" style={SANS}>
          HENRY DAVID THOREAU
        </text>
      </>
    ),
  },

  dracula: {
    glow: ["#3a0f1c", "#e25d5d"],
    art: (
      <>
        <rect width="200" height="300" fill="#120c14" />
        <circle cx="118" cy="104" r="50" fill="#e9dccb" />
        <path
          d="M 0 300 L 0 200 L 22 200 L 22 170 L 30 160 L 38 170 L 38 186 L 64 186 L 64 140 L 72 126 L 80 140 L 80 186 L 104 186 L 104 162 L 120 162 L 120 132 L 130 116 L 140 132 L 140 176 L 166 176 L 166 156 L 174 146 L 182 156 L 182 196 L 200 196 L 200 300 Z"
          fill="#2a0f17"
        />
        <g fill="#2a0f17">
          <path d="M 66 70 q 6 -6 10 0 q 4 -6 10 0 q -5 0 -10 5 q -5 -5 -10 -5 z" />
          <path d="M 40 96 q 4 -4 7 0 q 3 -4 7 0 q -3 0 -7 3.5 q -4 -3.5 -7 -3.5 z" />
        </g>
        <text x="100" y="250" textAnchor="middle" fontSize="23" letterSpacing="5" fill="#e25d5d" style={SERIF}>
          DRACULA
        </text>
        <text x="100" y="270" textAnchor="middle" fontSize="7.5" letterSpacing="2.6" fill="#c9b9b0" style={SANS}>
          BRAM STOKER
        </text>
      </>
    ),
  },

  pride: {
    glow: ["#34506e", "#dfe7ef"],
    art: (
      <>
        <rect width="200" height="300" fill="#dce5ee" />
        <ellipse cx="100" cy="138" rx="70" ry="94" fill="none" stroke="#34506e" strokeWidth="1.6" />
        <ellipse cx="100" cy="138" rx="64" ry="88" fill="none" stroke="#34506e" strokeWidth="0.7" />
        <g fill="none" stroke="#7a9a6a" strokeWidth="1.4" strokeLinecap="round">
          <path d="M 100 196 C 98 180 102 168 100 156" />
        </g>
        <g fill="#7a9a6a">
          <ellipse cx="94" cy="182" rx="6" ry="2.4" transform="rotate(-30 94 182)" />
          <ellipse cx="106" cy="172" rx="6" ry="2.4" transform="rotate(30 106 172)" />
        </g>
        <circle cx="100" cy="152" r="5" fill="#d98a8a" />
        <g textAnchor="middle" fontSize="21" fontStyle="italic" fill="#34506e" style={SERIF}>
          <text x="100" y="110">Pride and</text>
          <text x="100" y="134">Prejudice</text>
        </g>
        <text x="100" y="270" textAnchor="middle" fontSize="7.5" letterSpacing="2.6" fill="#34506e" style={SANS}>
          JANE AUSTEN
        </text>
      </>
    ),
  },
};
