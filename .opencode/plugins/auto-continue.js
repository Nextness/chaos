// auto-continue plugin
// Automatically sends "continue" when a session error with HTTP status >= 500
// is detected. Use /auto-continue to toggle it on/off at runtime.

const MAX_RETRIES = 3;
const RETRY_DELAY_MS = 1500;

export const AutoContinue = async ({ client }) => {
  let enabled = true;
  const retryCounts = new Map();

  const log = (level, message) => {
    try {
      client?.app?.log({ body: { service: 'auto-continue', level, message } });
    } catch (_) {}
  };

  const cleanupStale = () => {
    const cutoff = Date.now() - 300_000;
    for (const [key, entry] of retryCounts) {
      if (entry.timestamp < cutoff) retryCounts.delete(key);
    }
  };

  return {
    // Register the /auto-continue slash command so it appears in the palette.
    config: async (config) => {
      if (!config.command) config.command = {};
      config.command['auto-continue'] = {
        description: 'Toggle auto-continue on API timeouts (HTTP >= 500)',
        template: 'Toggle auto-continue',
      };
    },

    // Intercept /auto-continue to toggle the flag and report the new state.
    'command.execute.before': async (input, output) => {
      if (!input || input.command !== 'auto-continue') return;

      const arg = (input.arguments || '').trim().toLowerCase();
      if (arg === 'on') enabled = true;
      else if (arg === 'off') enabled = false;
      else enabled = !enabled;

      const status = enabled ? 'enabled' : 'disabled';
      log('info', status);

      // Replace the template message with a status notice.
      output.parts = [
        {
          type: 'text',
          text: `Auto-continue is now **${status}**.`,
        },
      ];
    },

    // React to session errors only when enabled.
    event: async ({ event }) => {
      if (!enabled) return;
      if (event.type !== 'session.error') return;

      const { sessionID, error } = event.properties;
      if (!sessionID || !error) return;
      if (error.name !== 'APIError') return;

      const { statusCode, isRetryable } = error.data;
      if (statusCode === undefined || statusCode < 500) return;
      if (!isRetryable) return;

      cleanupStale();

      const entry =
        retryCounts.get(sessionID) || { count: 0, timestamp: Date.now() };
      if (entry.count >= MAX_RETRIES) return;
      entry.count += 1;
      entry.timestamp = Date.now();
      retryCounts.set(sessionID, entry);

      await new Promise((r) => setTimeout(r, RETRY_DELAY_MS));

      try {
        await client.session.prompt({
          sessionID,
          parts: [{ type: 'text', text: 'continue' }],
        });
      } catch (err) {
        console.error('[auto-continue] failed to send continue:', err);
      }
    },
  };
};