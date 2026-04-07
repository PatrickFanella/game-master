import { useCallback, useState } from 'react';
import { cn } from '../../lib/cn';

interface CombatActionBarProps {
  readonly onAction: (text: string) => void;
  readonly disabled: boolean;
}

type ExpandedInput = 'spell' | 'item' | null;

const ACTION_BUTTON_STYLE =
  'inline-flex items-center justify-center border-2 border-ruby/30 bg-charcoal px-4 py-2 text-xs font-semibold uppercase tracking-[0.15em] text-champagne transition-all duration-200 hover:border-ruby hover:bg-ruby/10 hover:text-ruby focus:outline-none focus:ring-2 focus:ring-ruby focus:ring-offset-2 focus:ring-offset-obsidian disabled:cursor-not-allowed disabled:opacity-40';

export function CombatActionBar({ onAction, disabled }: CombatActionBarProps) {
  const [expandedInput, setExpandedInput] = useState<ExpandedInput>(null);
  const [inputText, setInputText] = useState('');

  const handleAction = useCallback(
    (text: string) => {
      if (!disabled) {
        onAction(text);
      }
    },
    [disabled, onAction],
  );

  const handleExpandedSubmit = useCallback(() => {
    const trimmed = inputText.trim();
    if (!trimmed || disabled) return;

    if (expandedInput === 'spell') {
      handleAction(`I cast ${trimmed}`);
    } else if (expandedInput === 'item') {
      handleAction(`I use ${trimmed}`);
    }

    setInputText('');
    setExpandedInput(null);
  }, [disabled, expandedInput, handleAction, inputText]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        e.preventDefault();
        handleExpandedSubmit();
      } else if (e.key === 'Escape') {
        setExpandedInput(null);
        setInputText('');
      }
    },
    [handleExpandedSubmit],
  );

  return (
    <div className="border-2 border-ruby/20 bg-charcoal px-4 py-3">
      <h3 className="mb-3 font-heading text-xs font-semibold uppercase tracking-[0.2em] text-ruby">
        Combat Actions
      </h3>

      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          className={ACTION_BUTTON_STYLE}
          disabled={disabled}
          onClick={() => handleAction('I attack the nearest enemy')}
        >
          Attack
        </button>
        <button
          type="button"
          className={ACTION_BUTTON_STYLE}
          disabled={disabled}
          onClick={() => handleAction('I take the Dodge action')}
        >
          Defend
        </button>
        <button
          type="button"
          className={cn(ACTION_BUTTON_STYLE, expandedInput === 'spell' && 'border-ruby bg-ruby/10 text-ruby')}
          disabled={disabled}
          onClick={() => {
            setExpandedInput(expandedInput === 'spell' ? null : 'spell');
            setInputText('');
          }}
        >
          Cast Spell
        </button>
        <button
          type="button"
          className={cn(ACTION_BUTTON_STYLE, expandedInput === 'item' && 'border-ruby bg-ruby/10 text-ruby')}
          disabled={disabled}
          onClick={() => {
            setExpandedInput(expandedInput === 'item' ? null : 'item');
            setInputText('');
          }}
        >
          Use Item
        </button>
        <button
          type="button"
          className={ACTION_BUTTON_STYLE}
          disabled={disabled}
          onClick={() => handleAction('I attempt to flee from combat')}
        >
          Flee
        </button>
      </div>

      {expandedInput !== null ? (
        <div className="mt-3 flex gap-2">
          <input
            type="text"
            value={inputText}
            onChange={(e) => setInputText(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={expandedInput === 'spell' ? 'Enter spell name...' : 'Enter item name...'}
            disabled={disabled}
            autoFocus
            className="flex-1 border-2 border-ruby/20 bg-obsidian px-3 py-2 text-sm text-champagne placeholder:text-pewter/50 focus:border-ruby focus:outline-none disabled:opacity-40"
          />
          <button
            type="button"
            className={ACTION_BUTTON_STYLE}
            disabled={disabled || inputText.trim().length === 0}
            onClick={handleExpandedSubmit}
          >
            Send
          </button>
          <button
            type="button"
            className="inline-flex items-center justify-center border-2 border-pewter/30 px-3 py-2 text-xs font-semibold uppercase tracking-[0.15em] text-pewter transition-all duration-200 hover:border-pewter hover:text-champagne"
            onClick={() => {
              setExpandedInput(null);
              setInputText('');
            }}
          >
            Cancel
          </button>
        </div>
      ) : null}
    </div>
  );
}
