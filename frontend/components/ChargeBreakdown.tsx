import { formatINR } from "@/lib/status";

export type ChargeBreakdownData = {
  chargeable_weight_kg: number;
  charge_breakdown: {
    base_charge: number;
    cod_surcharge: number;
    fuel_surcharge: number;
    gst_amount: number;
    total_charge: number;
  };
};

export function ChargeBreakdownCard({
  data,
  onConfirm,
  confirming = false,
}: {
  data: ChargeBreakdownData;
  onConfirm?: () => void;
  confirming?: boolean;
}) {
  const cb = data.charge_breakdown;

  return (
    <div className="rounded-lg border border-zinc-200 bg-white p-6 shadow-sm max-w-sm">
      <h2 className="text-lg font-semibold mb-4">Charge breakdown</h2>
      <dl className="text-sm space-y-2">
        <Row label={`Chargeable weight: ${data.chargeable_weight_kg} kg`} />
        <Row label="Base charge" value={formatINR(cb.base_charge)} />
        <Row label="COD surcharge" value={formatINR(cb.cod_surcharge)} />
        <Row label="Fuel surcharge" value={formatINR(cb.fuel_surcharge)} />
        <Row label="GST" value={formatINR(cb.gst_amount)} />
        <hr className="my-3 border-zinc-300" />
        <div className="flex justify-between font-semibold text-base">
          <span>Total</span>
          <span>{formatINR(cb.total_charge)}</span>
        </div>
      </dl>
      {onConfirm && (
        <button
          onClick={onConfirm}
          disabled={confirming}
          className="mt-5 w-full rounded-md bg-black py-2.5 text-white text-sm font-medium hover:bg-zinc-800 disabled:opacity-50"
        >
          {confirming ? "Confirming…" : "Confirm Order"}
        </button>
      )}
    </div>
  );
}

function Row({ label, value }: { label: string; value?: string }) {
  return (
    <div className="flex justify-between">
      <dt className="text-zinc-600">{label}</dt>
      {value && <dd>{value}</dd>}
    </div>
  );
}
