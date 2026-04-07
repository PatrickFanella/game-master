export type RulesMode = 'narrative' | 'light' | 'crunch';

interface RulesModeOption {
  readonly value: RulesMode;
  readonly title: string;
  readonly eyebrow: string;
  readonly description: string;
}

const rulesModeOptions: readonly RulesModeOption[] = [
  {
    value: 'narrative',
    title: 'Narrative',
    eyebrow: 'Pure storytelling',
    description: 'Pure storytelling. No dice, no HP tracking. The world responds to your words.',
  },
  {
    value: 'light',
    title: 'Light Rules',
    eyebrow: 'Balanced play',
    description: 'Balanced play. Dice rolls add tension, HP matters, but the story leads.',
  },
  {
    value: 'crunch',
    title: 'Crunch',
    eyebrow: 'Full mechanics',
    description: 'Full mechanics. D&D 5e-style feats, skills, initiative, and tactical combat.',
  },
];

interface RulesModeStepProps {
  readonly selectedMode: RulesMode | null;
  readonly onSelectMode: (mode: RulesMode) => void;
  readonly onContinue: () => void;
  readonly onBack?: () => void;
  readonly errorMessage?: string | null;
  readonly isLoading?: boolean;
}

export function RulesModeStep({
  selectedMode,
  onSelectMode,
  onContinue,
  onBack,
  errorMessage = null,
  isLoading = false,
}: RulesModeStepProps) {
  return (
    <section className="deco-corners deco-pattern space-y-6 border-2 border-gold/20 bg-charcoal p-6">
      <header className="space-y-3 border-b-2 border-gold/20 pb-5">
        <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-gold">
          Rules mode
        </p>
        <div className="space-y-2">
          <h2 className="font-heading text-2xl font-semibold uppercase tracking-wide text-champagne">
            Choose the rules mode
          </h2>
          <p className="max-w-2xl text-sm leading-7 text-champagne/70">
            Decide how much mechanical crunch the campaign uses. This shapes which tools the game
            master has available and how encounters play out.
          </p>
        </div>
      </header>

      <div className="grid gap-4 md:grid-cols-3">
        {rulesModeOptions.map((option) => {
          const isSelected = selectedMode === option.value;

          return (
            <button
              key={option.value}
              type="button"
              onClick={() => onSelectMode(option.value)}
              aria-pressed={isSelected}
              className={[
                'border-2 px-5 py-6 text-left transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-gold/60',
                isSelected
                  ? 'border-gold bg-gold/10'
                  : 'border-charcoal bg-obsidian hover:border-gold/40 hover:bg-charcoal hover:-translate-y-0.5',
              ].join(' ')}
            >
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <p className="font-heading text-[10px] font-semibold uppercase tracking-[0.25em] text-pewter/80">
                    {option.eyebrow}
                  </p>
                  <span
                    aria-hidden="true"
                    className={[
                      'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full border text-xs font-semibold',
                      isSelected
                        ? 'border-gold bg-gold text-obsidian'
                        : 'border-gold/30 bg-obsidian text-pewter',
                    ].join(' ')}
                  >
                    {isSelected ? '\u2713' : ''}
                  </span>
                </div>
                <h3 className="font-heading text-xl font-semibold uppercase tracking-wide text-champagne">
                  {option.title}
                </h3>
                <p className="text-sm leading-6 text-champagne/70">{option.description}</p>
              </div>
            </button>
          );
        })}
      </div>

      {!selectedMode && (
        <div className="border border-gold/40 bg-gold/10 px-4 py-3 text-sm text-gold">
          Select a rules mode before continuing.
        </div>
      )}

      {errorMessage && (
        <div className="border border-ruby/40 bg-ruby/10 px-4 py-3 text-sm text-ruby">
          {errorMessage}
        </div>
      )}

      <div className="flex flex-col-reverse gap-3 border-t-2 border-gold/20 pt-5 sm:flex-row sm:items-center sm:justify-between">
        {onBack ? (
          <button
            type="button"
            onClick={onBack}
            className="inline-flex items-center justify-center border border-gold/30 px-4 py-3 text-sm font-semibold uppercase tracking-wide text-champagne/80 transition hover:border-gold hover:text-champagne focus:outline-none focus:ring-2 focus:ring-gold/60"
          >
            Back
          </button>
        ) : (
          <div />
        )}
        <button
          type="button"
          onClick={onContinue}
          disabled={!selectedMode || isLoading}
          className="inline-flex items-center justify-center bg-ruby px-5 py-3 text-sm font-semibold uppercase tracking-wide text-champagne transition hover:bg-ruby-light focus:outline-none focus:ring-2 focus:ring-ruby focus:ring-offset-2 focus:ring-offset-obsidian disabled:cursor-not-allowed disabled:bg-charcoal disabled:text-pewter"
        >
          {isLoading ? 'Continuing\u2026' : 'Continue'}
        </button>
      </div>
    </section>
  );
}
