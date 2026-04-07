import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { getCampaignCharacterSkills } from '../../api/characters';
import { cn } from '../../lib/cn';

interface SkillTreeProps {
  readonly campaignId: string;
  readonly className?: string;
}

export function SkillTree({ campaignId, className }: SkillTreeProps) {
  const [isExpanded, setIsExpanded] = useState(false);

  const skillsQuery = useQuery({
    queryKey: ['campaign', campaignId, 'character', 'skills'],
    queryFn: () => getCampaignCharacterSkills(campaignId),
    enabled: campaignId.length > 0,
  });

  return (
    <section className={cn('border-2 border-jade/20 bg-charcoal', className)}>
      <button
        type="button"
        onClick={() => setIsExpanded(!isExpanded)}
        className="flex w-full items-center justify-between p-6 text-left transition hover:bg-jade/5"
      >
        <div>
          <p className="font-heading text-xs font-semibold uppercase tracking-[0.2em] text-jade">Crunch</p>
          <h3 className="font-heading mt-1 text-lg font-semibold uppercase tracking-[0.1em] text-champagne">
            Skills
          </h3>
        </div>
        <span className="text-sm text-pewter">{isExpanded ? '\u25B2' : '\u25BC'}</span>
      </button>

      {isExpanded && (
        <div className="border-t border-jade/15 px-6 pb-6">
          {skillsQuery.isPending && (
            <p className="py-4 text-sm text-pewter">Loading skills...</p>
          )}

          {skillsQuery.isError && (
            <p className="py-4 text-sm text-ruby">Failed to load skills.</p>
          )}

          {skillsQuery.isSuccess && skillsQuery.data.length === 0 && (
            <div className="mt-4 border border-dashed border-jade/15 bg-charcoal/50 px-4 py-6 text-sm text-pewter">
              No skill points have been allocated yet. Skills are improved through gameplay in crunch mode.
            </div>
          )}

          {skillsQuery.isSuccess && skillsQuery.data.length > 0 && (
            <ul className="mt-4 space-y-3">
              {skillsQuery.data.map((skill) => (
                <li
                  key={skill.id}
                  className="flex items-center justify-between border border-jade/20 bg-charcoal/80 p-4 transition-all duration-200 hover:border-sapphire/30"
                >
                  <div className="space-y-1">
                    <h4 className="text-base font-semibold text-champagne">{skill.name}</h4>
                    <p className="text-xs uppercase tracking-[0.15em] text-pewter/80">
                      {skill.base_ability} &middot; {skill.description}
                    </p>
                  </div>
                  <div className="flex items-center gap-1">
                    {Array.from({ length: Math.min(skill.points, 5) }).map((_, i) => (
                      <span
                        key={`${skill.id}-pip-${i}`}
                        className="inline-block h-3 w-3 rounded-full bg-sapphire"
                      />
                    ))}
                    {skill.points > 5 && (
                      <span className="ml-1 text-xs font-semibold text-sapphire">
                        +{skill.points - 5}
                      </span>
                    )}
                    <span className="ml-2 text-sm font-semibold text-champagne/80">
                      {skill.points}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </section>
  );
}
