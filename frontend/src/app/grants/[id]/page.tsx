import GrantDetailClient from './GrantDetailClient';

// Required for Next.js static export (output: 'export').
// We generate a single placeholder page; the Go server's SPA fallback
// serves this HTML for any /grants/<uuid> route and the client-side
// router picks up the actual ID from the URL.
export async function generateStaticParams() {
  return [{ id: '_' }];
}

export default function GrantDetailPage({
  params,
}: {
  params: { id: string };
}) {
  return <GrantDetailClient params={params} />;
}
