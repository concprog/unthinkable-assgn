import {
  allMainFlow,
  mainFlowIndex,
  statusLabel,
  type OrderStatus,
} from "@/lib/status";

type HistoryEntry = {
  status: string;
  created_at?: string;
};

const EXCEPTIONS: Record<string, string> = {
  FAILED: "✕ Failed",
  RESCHEDULED: "↻ Rescheduled",
  CANCELLED: "✕ Cancelled",
};

export function StatusTimeline({
  currentStatus,
  history = [],
}: {
  currentStatus: OrderStatus;
  history?: HistoryEntry[];
}) {
  const currentIdx = mainFlowIndex(currentStatus);
  const exception = EXCEPTIONS[currentStatus];

  const assignedEntry = history.find((h) => h.status === "ASSIGNED");

  return (
    <ol className="flex flex-col gap-3">
      {allMainFlow().map((status, idx) => {
        const done = currentIdx >= idx && !exception;
        return (
          <li key={status} className="flex items-center gap-3">
            <span
              className={`h-3 w-3 rounded-full border ${
                done ? "bg-green-600 border-green-600" : "bg-white border-zinc-400"
              }`}
            />
            <span className={done ? "text-black font-medium" : "text-zinc-400"}>
              {statusLabel(status)}
              {status === "ASSIGNED" && assignedEntry?.created_at && " — agent on the way"}
            </span>
          </li>
        );
      })}
      {exception && (
        <li className="flex items-center gap-3">
          <span className="h-3 w-3 rounded-full bg-red-600" />
          <span className="text-red-600 font-medium">{exception}</span>
        </li>
      )}
    </ol>
  );
}
