import { useState } from 'react';
import { useNavigate } from 'react-router';

import { useAuth } from '../../context/AuthContext';

export function UserMenu() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);

  if (!user) return null;

  async function handleLogout() {
    setOpen(false);
    await logout();
    navigate('/login', { replace: true });
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className="flex items-center gap-2 border border-gold/20 px-3 py-2 text-xs font-semibold uppercase tracking-[0.15em] text-gold transition-all duration-200 hover:border-gold hover:text-gold-light"
      >
        <span className="inline-flex h-6 w-6 items-center justify-center border border-gold/40 bg-gold/10 text-xs font-bold text-gold">
          {user.name.charAt(0).toUpperCase()}
        </span>
        <span className="hidden sm:inline">{user.name}</span>
      </button>

      {open && (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => setOpen(false)}
          />
          <div className="absolute right-0 z-50 mt-2 w-48 border-2 border-gold/20 bg-charcoal shadow-gold">
            <div className="border-b border-gold/10 px-4 py-3">
              <p className="text-sm font-medium text-champagne">{user.name}</p>
              <p className="text-xs text-pewter">{user.email}</p>
            </div>
            <button
              type="button"
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
                void handleLogout();
              }}
              className="w-full px-4 py-3 text-left text-sm text-champagne/80 transition-colors hover:bg-ruby/10 hover:text-ruby"
            >
              Sign out
            </button>
          </div>
        </>
      )}
    </div>
  );
}
