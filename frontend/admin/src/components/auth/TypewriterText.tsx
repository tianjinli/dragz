import { useMemo } from 'react';

/** How long the fully typed title stays on screen before it restarts. */
const PAUSE_MS = 2500;
/** On/off interval for the caret while the title is fully shown. */
const CARET_BLINK_MS = 500;
/** Sit flush after the last glyph — real caret spacing, not a padded gap. */
const CARET_END = '100%';

/**
 * Reveals text one character at a time, pauses once complete, then replays.
 * The full text stays in the DOM so screen readers and tests see it immediately;
 * the typewriter effect is purely visual via clip-path keyframes generated here
 * (step count and pause scale with the actual character count).
 */
export function TypewriterText({ text }: { text: string }) {
  const count = [...text].length;
  const typingMs = Math.min(count * 55, 2600);
  const totalMs = typingMs + PAUSE_MS;
  const typedPct = (typingMs / totalMs) * 100;
  const keyframeId = `auth-tw-${count}-${typingMs}`;

  const css = useMemo(() => {
    const caretFrames: string[] = [
      `0% { left: 0; opacity: 1; animation-timing-function: steps(${count}, end); }`,
      `${typedPct.toFixed(2)}% { left: ${CARET_END}; opacity: 1; animation-timing-function: linear; }`,
    ];
    let t = typingMs;
    let on = true;
    while (t < totalMs - CARET_BLINK_MS) {
      t += CARET_BLINK_MS;
      caretFrames.push(
        `${Math.min((t / totalMs) * 100, 99).toFixed(2)}% { left: ${CARET_END}; opacity: ${on ? 0 : 1}; }`,
      );
      on = !on;
    }
    caretFrames.push(
      `99% { left: ${CARET_END}; opacity: 1; animation-timing-function: steps(1, end); }`,
      '100% { left: 0; opacity: 0; }',
    );

    return [
      `@keyframes ${keyframeId}-text {`,
      `  0% { clip-path: inset(0 100% 0 0); animation-timing-function: steps(${count}, end); }`,
      `  ${typedPct.toFixed(2)}% { clip-path: inset(0 0 0 0); animation-timing-function: linear; }`,
      '  99% { clip-path: inset(0 0 0 0); animation-timing-function: steps(1, end); }',
      '  100% { clip-path: inset(0 100% 0 0); }',
      '}',
      `@keyframes ${keyframeId}-caret {`,
      ...caretFrames.map((frame) => `  ${frame}`),
      '}',
    ].join('\n');
  }, [count, typingMs, totalMs, typedPct, keyframeId]);

  return (
    <span className="auth-typewriter">
      <style>{css}</style>
      <span
        className="auth-typewriter-text"
        style={{ animation: `${keyframeId}-text ${totalMs}ms linear infinite` }}
      >
        {text}
      </span>
      <span
        aria-hidden="true"
        className="auth-typewriter-caret"
        style={{ animation: `${keyframeId}-caret ${totalMs}ms linear infinite` }}
      />
    </span>
  );
}
