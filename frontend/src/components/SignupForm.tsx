import { useState, type FormEvent } from 'react';
import { useAuthStore } from '../lib/store';

export default function SignupForm() {
  const { signup } = useAuthStore();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [referralCode, setReferralCode] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      await signup(email, password, firstName, lastName, referralCode || undefined);
      window.location.href = '/app';
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Signup failed. Please try again.');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center px-4 py-8">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="flex items-center justify-center gap-2 mb-4">
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              className="w-8 h-8 text-terminal-accent"
            >
              <polyline points="22 7 13.5 15.5 8.5 10.5 2 17" />
              <polyline points="16 7 22 7 22 13" />
            </svg>
            <h1 className="text-2xl font-bold text-terminal-accent tracking-tight">
              STOCKS
            </h1>
          </div>
          <p className="text-terminal-muted text-sm">Create your terminal account</p>
        </div>

        {/* Form */}
        <form
          onSubmit={handleSubmit}
          className="bg-terminal-panel border border-terminal-border rounded-lg p-6 space-y-4"
        >
          {error && (
            <div className="bg-terminal-red/10 border border-terminal-red/30 rounded px-3 py-2 text-sm text-terminal-red">
              {error}
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label htmlFor="firstName" className="block text-xs text-terminal-muted mb-1.5 uppercase tracking-wider">
                First Name
              </label>
              <input
                id="firstName"
                type="text"
                value={firstName}
                onChange={(e) => setFirstName(e.target.value)}
                required
                autoComplete="given-name"
                className="terminal-input w-full"
                placeholder="Jane"
              />
            </div>
            <div>
              <label htmlFor="lastName" className="block text-xs text-terminal-muted mb-1.5 uppercase tracking-wider">
                Last Name
              </label>
              <input
                id="lastName"
                type="text"
                value={lastName}
                onChange={(e) => setLastName(e.target.value)}
                required
                autoComplete="family-name"
                className="terminal-input w-full"
                placeholder="Doe"
              />
            </div>
          </div>

          <div>
            <label htmlFor="email" className="block text-xs text-terminal-muted mb-1.5 uppercase tracking-wider">
              Email
            </label>
            <input
              id="email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
              className="terminal-input w-full"
              placeholder="you@example.com"
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-xs text-terminal-muted mb-1.5 uppercase tracking-wider">
              Password
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="new-password"
              className="terminal-input w-full"
              placeholder="Minimum 8 characters"
              minLength={8}
            />
          </div>

          <div>
            <label htmlFor="referral" className="block text-xs text-terminal-muted mb-1.5 uppercase tracking-wider">
              Referral Code <span className="text-terminal-muted/50">(optional)</span>
            </label>
            <input
              id="referral"
              type="text"
              value={referralCode}
              onChange={(e) => setReferralCode(e.target.value)}
              autoComplete="off"
              className="terminal-input w-full"
              placeholder="Enter referral code"
            />
          </div>

          <button
            type="submit"
            disabled={isSubmitting}
            className="btn-primary w-full mt-2"
          >
            {isSubmitting ? 'Creating account...' : 'Create Account'}
          </button>

          <p className="text-center text-sm text-terminal-muted">
            Already have an account?{' '}
            <a href="/login" className="text-terminal-accent hover:underline">
              Sign in
            </a>
          </p>
        </form>
      </div>
    </div>
  );
}
