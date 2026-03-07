'use client';

import { createContext, useContext, useCallback } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { api, SessionInfo } from '@/lib/api';

interface AuthContextType {
  session: SessionInfo | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isAdmin: boolean;
  isGrantAdmin: boolean;
  isSubawardAdmin: boolean;
  isReadOnly: boolean;
  /** Institutions a subaward_admin may access; empty for other roles */
  permittedInstitutions: string[];
  refetch: () => void;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
  session: null,
  isLoading: true,
  isAuthenticated: false,
  isAdmin: false,
  isGrantAdmin: false,
  isSubawardAdmin: false,
  isReadOnly: false,
  permittedInstitutions: [],
  refetch: () => {},
  logout: async () => {},
});

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();

  const {
    data: session,
    isLoading,
    refetch,
  } = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: async () => {
      try {
        return await api.auth.me();
      } catch {
        return null;
      }
    },
    retry: false,
    staleTime: 60_000,
  });

  const logout = useCallback(async () => {
    try {
      await api.auth.logout();
    } catch {
      // ignore
    }
    queryClient.setQueryData(['auth', 'me'], null);
    queryClient.invalidateQueries();
    window.location.href = '/login';
  }, [queryClient]);

  const isAuthenticated = !!session?.user && session.user.status === 'active';
  const isAdmin = session?.role === 'admin';
  const isGrantAdmin = session?.role === 'grant_admin';
  const isSubawardAdmin = session?.role === 'subaward_admin';
  const isReadOnly = session?.role === 'read_only';
  const permittedInstitutions = session?.institutions ?? [];

  return (
    <AuthContext.Provider
      value={{
        session: session ?? null,
        isLoading,
        isAuthenticated,
        isAdmin,
        isGrantAdmin,
        isSubawardAdmin,
        isReadOnly,
        permittedInstitutions,
        refetch: () => refetch(),
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
