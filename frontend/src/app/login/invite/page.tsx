import { Suspense } from 'react';
import InviteClient from './InviteClient';

export default function InvitePage() {
  return (
    <Suspense
      fallback={
        <div className="flex items-center justify-center min-h-screen bg-gray-50">
          <div className="text-gray-500">Loading...</div>
        </div>
      }
    >
      <InviteClient />
    </Suspense>
  );
}
