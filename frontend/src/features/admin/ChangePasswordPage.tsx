import { FormEvent, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import Panel from '@/shared/Panel';
import Input from '@/shared/Input';
import Button from '@/shared/Button';
import apiClient from '@/playground/apiClient.js';
import { getSessionState, setToken } from '@/playground/auth.js';

const ChangePasswordPage = () => {
  const navigate = useNavigate();
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    const checkSession = async () => {
      const session = await getSessionState();
      if (!active) return;
      setChecking(false);
      if (!session.valid) {
        navigate('/playground/login', { replace: true });
        return;
      }
      if (!session.mustChangePassword) {
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
    if (!oldPassword || !newPassword || !confirmPassword) {
      setError('All fields are required.');
      return;
    }
    if (newPassword.length < 8) {
      setError('New password must be at least 8 characters.');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('Confirm password does not match.');
      return;
    }

    setLoading(true);
    setError('');
    try {
      const { data } = await apiClient.post(
        '/auth/change-password',
        { old_password: oldPassword, new_password: newPassword },
        { skipUnauthorizedRedirect: true }
      );
      if (data?.access_token) {
        setToken(data.access_token);
      }
      navigate('/playground', { replace: true });
    } catch (err) {
      if (axios.isAxiosError(err)) {
        setError(err.response?.data?.error?.message || 'Failed to change password.');
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Failed to change password.');
      }
    } finally {
      setLoading(false);
    }
  };

  if (checking) {
    return (
      <Panel title="Change Password">
        <div className="badge">Checking session...</div>
      </Panel>
    );
  }

  return (
    <Panel title="Change Password">
      <form className="grid" onSubmit={handleSubmit}>
        <Input
          label="Old Password"
          type="password"
          value={oldPassword}
          onChange={(e) => setOldPassword(e.target.value)}
          autoComplete="current-password"
        />
        <Input
          label="New Password"
          type="password"
          value={newPassword}
          onChange={(e) => setNewPassword(e.target.value)}
          autoComplete="new-password"
        />
        <Input
          label="Confirm New Password"
          type="password"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          autoComplete="new-password"
        />
        {error && <div className="badge">{error}</div>}
        <Button type="submit" disabled={loading}>
          {loading ? 'Updating...' : 'Update Password'}
        </Button>
      </form>
    </Panel>
  );
};

export default ChangePasswordPage;
