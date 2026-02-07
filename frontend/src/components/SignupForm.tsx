import { useState } from 'react';
import { signup } from '@/lib/api';

export default function SignupForm() {
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [referralCode, setReferralCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const passwordsMatch = password === confirmPassword;
  const passwordLongEnough = password.length >= 8;
  const canSubmit =
    firstName &&
    lastName &&
    email &&
    password &&
    confirmPassword &&
    referralCode &&
    passwordsMatch &&
    passwordLongEnough &&
    !loading;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!passwordsMatch) {
      setError('Passwords do not match.');
      return;
    }

    if (!passwordLongEnough) {
      setError('Password must be at least 8 characters.');
      return;
    }

    setLoading(true);

    try {
      await signup({
        first_name: firstName,
        last_name: lastName,
        email,
        password,
        referral_code: referralCode.trim(),
      });
      window.location.href = '/app';
    } catch (err: unknown) {
      const message =
        err instanceof Error
          ? err.message
          : 'Signup failed. Please check your referral code and try again.';
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center px-4 py-8 bg-terminal-bg">
      <div className="w-full max-w-sm">
        {/* Header */}
        <div className="text-center mb-8">
          <div className="flex items-center justify-center gap-2 mb-4">
            <svg
              className="w-8 h-8 text-terminal-accent"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <polyline points="22 7 13.5 15.5 8.5 10.5 2 17" />
              <polyline points="16 7 22 7 22 13" />
            </svg>
            <span className="text-2xl font-mono font-bold text-terminal-accent tracking-tight">
              STOCKS
            </span>
          </div>
          <p className="text-terminal-dim text-sm">
            Create an account — referral code required
          </p>
        </div>

        {/* Form */}
        <div className="panel">
          <div className="p-6">
            <form onSubmit={handleSubmit} className="space-y-4">
              {error && (
                <div className="rounded-md bg-terminal-red/10 border border-terminal-red/20 px-3 py-2 text-sm text-terminal-red animate-fade-in">
                  {error}
                </div>
              )}

              {/* Name row */}
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label
                    htmlFor="firstName"
                    className="block text-xs font-medium text-terminal-dim uppercase tracking-wider mb-1.5"
                  >
                    First Name
                  </label>
                  <input
                    id="firstName"
                    type="text"
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    required
                    autoComplete="given-name"
                    autoFocus
                    placeholder="Jane"
                    className="input-field"
                  />
                </div>
                <div>
                  <label
                    htmlFor="lastName"
                    className="block text-xs font-medium text-terminal-dim uppercase tracking-wider mb-1.5"
                  >
                    Last Name
                  </label>
                  <input
                    id="lastName"
                    type="text"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    required
                    autoComplete="family-name"
                    placeholder="Doe"
                    className="input-field"
                  />
                </div>
              </div>

              <div>
                <label
                  htmlFor="email"
                  className="block text-xs font-medium text-terminal-dim uppercase tracking-wider mb-1.5"
                >
                  Email
                </label>
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoComplete="email"
                  placeholder="you@example.com"
                  className="input-field font-mono"
                />
              </div>

              <div>
                <label
                  htmlFor="password"
                  className="block text-xs font-medium text-terminal-dim uppercase tracking-wider mb-1.5"
                >
                  Password
                </label>
                <input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  autoComplete="new-password"
                  placeholder="••••••••"
                  minLength={8}
                  className="input-field font-mono"
                />
                {password && !passwordLongEnough && (
                  <p className="mt-1 text-2xs text-terminal-red">
                    Must be at least 8 characters
                  </p>
                )}
              </div>

              <div>
                <label
                  htmlFor="confirmPassword"
                  className="block text-xs font-medium text-terminal-dim uppercase tracking-wider mb-1.5"
                >
                  Confirm Password
                </label>
                <input
                  id="confirmPassword"
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  required
                  autoComplete="new-password"
                  placeholder="••••••••"
                  className={`input-field font-mono ${
                    confirmPassword && !passwordsMatch
                      ? 'border-terminal-red focus:border-terminal-red focus:ring-terminal-red'
                      : ''
                  }`}
                />
                {confirmPassword && !passwordsMatch && (
                  <p className="mt-1 text-2xs text-terminal-red">
                    Passwords do not match
                  </p>
                )}
              </div>

              {/* Referral Code — visually distinct */}
              <div className="pt-2 border-t border-terminal-border">
                <label
                  htmlFor="referralCode"
                  className="block text-xs font-medium text-terminal-accent uppercase tracking-wider mb-1.5"
                >
                  Referral Code
                </label>
                <input
                  id="referralCode"
                  type="text"
                  value={referralCode}
                  onChange={(e) => setReferralCode(e.target.value.toUpperCase())}
                  required
                  placeholder="ENTER-CODE"
                  className="input-field font-mono uppercase tracking-widest text-center
                             border-terminal-accent/30 focus:border-terminal-accent"
                />
                <p className="mt-1.5 text-2xs text-terminal-muted text-center">
                  A referral code is required to create an account
                </p>
              </div>

              <button
                type="submit"
                disabled={!canSubmit}
                className="btn-primary w-full"
              >
                {loading ? (
                  <span className="flex items-center gap-2">
                    <svg
                      className="animate-spin h-4 w-4"
                      viewBox="0 0 24 24"
                      fill="none"
                    >
                      <circle
                        className="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        strokeWidth="4"
                      />
                      <path
                        className="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                      />
                    </svg>
                    Creating account...
                  </span>
                ) : (
                  'Create Account'
                )}
              </button>
            </form>
          </div>
        </div>

        {/* Footer */}
        <p className="text-center text-xs text-terminal-muted mt-6">
          Already have an account?{' '}
          <a href="/login" className="text-terminal-accent hover:underline">
            Sign in
          </a>
        </p>
      </div>
    </div>
  );
}