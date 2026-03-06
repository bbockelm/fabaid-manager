'use client';

import { createContext, useContext } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api, Grant } from '@/lib/api';

interface GrantContextType {
  grant: Grant | null;
  grantId: string | null;
  isLoading: boolean;
  error: Error | null;
}

const GrantContext = createContext<GrantContextType>({
  grant: null,
  grantId: null,
  isLoading: true,
  error: null,
});

/**
 * Provides the single project grant to all child components.
 * Fetches the first grant from the API on mount.
 */
export function GrantProvider({ children }: { children: React.ReactNode }) {
  const {
    data: grants,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['grants'],
    queryFn: api.grants.list,
  });

  const grant = grants?.[0] ?? null;

  return (
    <GrantContext.Provider
      value={{
        grant,
        grantId: grant?.id ?? null,
        isLoading,
        error: error as Error | null,
      }}
    >
      {children}
    </GrantContext.Provider>
  );
}

export function useGrant() {
  return useContext(GrantContext);
}
