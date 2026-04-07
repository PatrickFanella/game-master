import { type FormEvent, useState } from 'react';
import { Link, useNavigate } from 'react-router';

import { register } from '../api/auth';
import { useAuth } from '../context/AuthContext';

export function RegisterPage() {
  const navigate = useNavigate();
  const { setSession } = useAuth();

  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);

    if (!name.trim() || !email.trim() || !password) {
      setError('All fields are required.');
      return;
    }
    if (password.length < 6) {
      setError('Password must be at least 6 characters.');
      return;
    }

    setIsSubmitting(true);
    try {
      const res = await register(name.trim(), email.trim(), password);
      setSession(res.token, res.user);
      navigate('/', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed.');
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
            Create Account
          </h1>
          <p className="text-sm leading-7 text-champagne/60">
            Register to begin your journey.
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
              Name
            </span>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full border-2 border-gold/20 bg-obsidian px-4 py-3 text-sm text-champagne transition-all duration-200 placeholder:text-pewter/60 focus:border-gold focus:outline-none focus:ring-2 focus:ring-gold/40"
              placeholder="Your adventurer name"
              autoComplete="name"
              autoFocus
            />
          </label>

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
              placeholder="At least 6 characters"
              autoComplete="new-password"
            />
          </label>

          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full bg-ruby px-5 py-3 text-sm font-semibold uppercase tracking-[0.15em] text-champagne transition-all duration-200 hover:bg-ruby-light hover:shadow-ruby focus:outline-none focus:ring-2 focus:ring-ruby focus:ring-offset-2 focus:ring-offset-obsidian disabled:cursor-not-allowed disabled:bg-charcoal disabled:text-pewter"
          >
            {isSubmitting ? 'Creating account…' : 'Create Account'}
          </button>
        </form>

        <p className="mt-6 text-center text-sm text-champagne/60">
          Already have an account?{' '}
          <Link
            to="/login"
            className="font-semibold text-gold transition-colors hover:text-gold-light"
          >
            Sign in
          </Link>
        </p>
      </div>
    </main>
  );
}
