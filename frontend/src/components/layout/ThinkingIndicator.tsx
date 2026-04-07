import type { WebSocketStatusPayload } from '../../api/types';
import { describeToolCall } from '../../lib/toolDescriptions';

interface ThinkingIndicatorProps {
  readonly status: WebSocketStatusPayload | null;
}

export function ThinkingIndicator({ status }: ThinkingIndicatorProps) {
  if (!status) {
    return null;
  }

  const description = status.tool ? describeToolCall(status.tool) : status.description;

  return (
    <div className="flex items-center gap-3 border-2 border-gold/20 bg-charcoal px-4 py-2.5 text-sm">
      <span className="relative flex h-2.5 w-2.5">
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-gold/60" />
        <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-gold" />
      </span>
      <span className="font-heading text-xs font-semibold uppercase tracking-[0.15em] text-champagne/80">
        {description}
      </span>
    </div>
  );
}
