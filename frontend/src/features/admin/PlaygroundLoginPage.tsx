import { FormEvent, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import Panel from '@/shared/Panel';
import Input from '@/shared/Input';
import Button from '@/shared/Button';
import { login, verifyToken } from '@/playground/auth.js';

const PlaygroundLoginPage = () => {
  const navigate = useNavigate();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    const checkSession = async () => {
      const valid = await verifyToken();
      if (!active) return;
      setChecking(false);
      if (valid) {
        navigate('/playground', { replace: true });
      }
    };
    void checkSession();
    return () => {
      active = false;
    };
  }, [navigate]);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!username.trim() || !password) {
      setError('Username and password are required.');
      return;
    }
    setLoading(true);
    setError('');
    try {
      await login(username.trim(), password);
      navigate('/playground', { replace: true });
    } catch (err) {
      if (axios.isAxiosError(err)) {
        setError(err.response?.data?.error?.message || 'Login failed.');
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Login failed.');
      }
    } finally {
      setLoading(false);
    }
  };

  if (checking) {
    return (
      <Panel title="Playground Login">
        <div className="badge">Checking session...</div>
      </Panel>
    );
  }

  return (
    <Panel title="Playground Login">
      <form className="grid" onSubmit={handleSubmit}>
        <Input label="Username" value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
        <Input
          label="Password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
        />
        {error && <div className="badge">{error}</div>}
        <Button type="submit" disabled={loading}>
          {loading ? 'Signing in...' : 'Sign in'}
        </Button>
      </form>
    </Panel>
  );
};

export default PlaygroundLoginPage;
