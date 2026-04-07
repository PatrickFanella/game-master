import { type FormEvent, useState } from 'react';
import { Link, useNavigate } from 'react-router';

import { login } from '../api/auth';
import { useAuth } from '../context/AuthContext';

export function LoginPage() {
  const navigate = useNavigate();
  const { setSession } = useAuth();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    if (!email.trim() || !password) {
      setError('Email and password are required.');
      return;
    }

    setIsSubmitting(true);
    try {
      const res = await login(email.trim(), password);
      setSession(res.token, res.user);
      navigate('/', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed.');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-obsidian px-6 text-champagne">
      <div className="deco-corners deco-pattern w-full max-w-md border-2 border-gold/20 bg-charcoal p-8">
        <div className="mb-8 space-y-3 text-center">
          <p className="font-heading text-sm font-semibold uppercase tracking-[0.32em] text-gold">
            Game Master
          </p>
          <h1 className="font-heading text-3xl font-semibold uppercase tracking-[0.12em]">
            Sign In
          </h1>
          <p className="text-sm leading-7 text-champagne/60">
            Enter your credentials to continue your adventure.
          </p>
        </div>

        {error && (
          <div className="mb-6 border border-ruby/40 bg-ruby/10 px-4 py-3 text-sm text-ruby">
            {error}
          </div>
        )}

        <form className="space-y-6" onSubmit={handleSubmit} noValidate>
          <label className="block space-y-2">
            <span className="text-xs font-semibold uppercase tracking-[0.2em] text-gold">
              Email
            </span>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="w-full border-2 border-gold/20 bg-obsidian px-4 py-3 text-sm text-champagne transition-all duration-200 placeholder:text-pewter/60 focus:border-gold focus:outline-none focus:ring-2 focus:ring-gold/40"
              placeholder="adventurer@example.com"
              autoComplete="email"
              autoFocus
            />
          </label>

          <label className="block space-y-2">
            <span className="text-xs font-semibold uppercase tracking-[0.2em] text-gold">
              Password
            </span>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full border-2 border-gold/20 bg-obsidian px-4 py-3 text-sm text-champagne transition-all duration-200 placeholder:text-pewter/60 focus:border-gold focus:outline-none focus:ring-2 focus:ring-gold/40"
              placeholder="Enter your password"
              autoComplete="current-password"
            />
          </label>

          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full bg-ruby px-5 py-3 text-sm font-semibold uppercase tracking-[0.15em] text-champagne transition-all duration-200 hover:bg-ruby-light hover:shadow-ruby focus:outline-none focus:ring-2 focus:ring-ruby focus:ring-offset-2 focus:ring-offset-obsidian disabled:cursor-not-allowed disabled:bg-charcoal disabled:text-pewter"
          >
            {isSubmitting ? 'Signing in…' : 'Sign In'}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-champagne/60">
          No account yet?{' '}
          <Link
            to="/register"
            className="font-semibold text-gold transition-colors hover:text-gold-light"
          >
            Create one
          </Link>
        </p>
      </div>
    </main>
  );
}
