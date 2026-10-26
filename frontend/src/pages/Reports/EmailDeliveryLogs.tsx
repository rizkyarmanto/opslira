import { Download } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import { DeliveryBadge } from '@/components/beautiful-ui/ProtocolBadge';
import { apiUrl } from '@/lib/api';
import { Head } from '@/lib/head';
import { formatDateTime } from '@/lib/time';
import type { EmailDeliveryLog } from '@/types/monitoring';

interface Props {
    logs: EmailDeliveryLog[];
    reports: Array<{
        id: string;
        period_type: string;
        period_start: string;
        generation_status: string;
        pdf_file_path: string | null;
        excel_file_path: string | null;
    }>;
}

const generationLabels: Record<string, string> = {
    COMPLETED: 'Completed',
    PENDING: 'Pending',
    PROCESSING: 'Processing',
    FAILED: 'Failed',
};

/**
 * Report history and email delivery log. Both tables are the record, so they stay
 * plain and scroll horizontally on small screens instead of collapsing (R-03).
 */
export default function EmailDeliveryLogs({ logs, reports }: Props) {
    return (
        <AppLayout title="Report & Email History">
            <Head title="Report & Email History" />

            <section className="mb-8">
                <h2 className="mb-3 text-sm font-semibold text-noc-text">Generated reports</h2>

                {reports.length === 0 ? (
                    <p className="rounded-md border border-dashed border-noc-border px-4 py-8 text-center text-xs text-noc-muted">
                        No reports generated yet. Run a report schedule to produce the first one.
                    </p>
                ) : (
                    <div className="overflow-x-auto rounded-md border border-noc-border">
                        <table className="w-full min-w-[560px] text-left text-sm">
                            <thead className="bg-white/[0.03]">
                                <tr>
                                    <th scope="col" className="field-label px-4 py-3">Period</th>
                                    <th scope="col" className="field-label px-4 py-3">Type</th>
                                    <th scope="col" className="field-label px-4 py-3">Generation status</th>
                                    <th scope="col" className="field-label px-4 py-3 text-right">Download</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-noc-border">
                                {reports.map((report) => (
                                    <tr key={report.id} className="hover:bg-white/[0.02]">
                                        <td className="tabular px-4 py-3 text-noc-text">
                                            {formatDateTime(report.period_start) ?? report.period_start}
                                        </td>
                                        <td className="px-4 py-3 text-noc-muted">{report.period_type}</td>
                                        <td className="px-4 py-3 text-noc-text">
                                            {generationLabels[report.generation_status] ?? report.generation_status}
                                        </td>
                                        <td className="px-4 py-3 text-right">
                                            <div className="inline-flex gap-3">
                                                {report.pdf_file_path ? (
                                                    <a
                                                        href={apiUrl(`/reports/download/${report.id}/pdf`)}
                                                        className="inline-flex items-center gap-1 text-xs text-noc-accent hover:underline"
                                                    >
                                                        <Download size={12} aria-hidden /> PDF
                                                    </a>
                                                ) : null}
                                                {report.excel_file_path ? (
                                                    <a
                                                        href={apiUrl(`/reports/download/${report.id}/xlsx`)}
                                                        className="inline-flex items-center gap-1 text-xs text-noc-accent hover:underline"
                                                    >
                                                        <Download size={12} aria-hidden /> XLSX
                                                    </a>
                                                ) : null}
                                                {!report.pdf_file_path && !report.excel_file_path && (
                                                    <span className="text-xs text-noc-muted">Not available yet</span>
                                                )}
                                            </div>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </section>

            <section>
                <h2 className="mb-3 text-sm font-semibold text-noc-text">Email delivery log</h2>

                {logs.length === 0 ? (
                    <p className="rounded-md border border-dashed border-noc-border px-4 py-8 text-center text-xs text-noc-muted">
                        No emails sent yet. Rows appear after a report or incident notification is delivered. Check MAIL settings and ALERT_RECIPIENTS.
                    </p>
                ) : (
                    <div className="overflow-x-auto rounded-md border border-noc-border">
                        <table className="w-full min-w-[720px] text-left text-sm">
                            <thead className="bg-white/[0.03]">
                                <tr>
                                    <th scope="col" className="field-label px-4 py-3">Time</th>
                                    <th scope="col" className="field-label px-4 py-3">Category</th>
                                    <th scope="col" className="field-label px-4 py-3">Recipient</th>
                                    <th scope="col" className="field-label px-4 py-3">Subject</th>
                                    <th scope="col" className="field-label px-4 py-3">Status</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-noc-border">
                                {logs.map((log) => (
                                    <tr key={log.id} className="hover:bg-white/[0.02]">
                                        <td className="tabular px-4 py-3 text-xs text-noc-muted">
                                            {formatDateTime(log.created_at) ?? ''}
                                        </td>
                                        <td className="px-4 py-3 text-xs text-noc-muted">{log.email_category}</td>
                                        <td className="tabular px-4 py-3 text-xs text-noc-text">{log.recipient_email}</td>
                                        <td className="px-4 py-3 text-xs text-noc-muted">{log.subject}</td>
                                        <td className="px-4 py-3">
                                            <DeliveryBadge status={log.delivery_status} />
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </section>
        </AppLayout>
    );
}
