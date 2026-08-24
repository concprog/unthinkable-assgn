export type OrderStatus =
  | "CREATED"
  | "CONFIRMED"
  | "ASSIGNED"
  | "PICKED_UP"
  | "IN_TRANSIT"
  | "OUT_FOR_DELIVERY"
  | "DELIVERED"
  | "FAILED"
  | "RESCHEDULED"
  | "CANCELLED";

const MAIN_FLOW: OrderStatus[] = [
  "CREATED",
  "CONFIRMED",
  "ASSIGNED",
  "PICKED_UP",
  "IN_TRANSIT",
  "OUT_FOR_DELIVERY",
  "DELIVERED",
];

const LABELS: Record<OrderStatus, string> = {
  CREATED: "Created",
  CONFIRMED: "Confirmed",
  ASSIGNED: "Assigned",
  PICKED_UP: "Picked Up",
  IN_TRANSIT: "In Transit",
  OUT_FOR_DELIVERY: "Out for Delivery",
  DELIVERED: "Delivered",
  FAILED: "Failed",
  RESCHEDULED: "Rescheduled",
  CANCELLED: "Cancelled",
};

const NEXT_LEGAL: Partial<Record<OrderStatus, OrderStatus[]>> = {
  CONFIRMED: ["ASSIGNED"],
  ASSIGNED: ["PICKED_UP"],
  PICKED_UP: ["IN_TRANSIT"],
  IN_TRANSIT: ["OUT_FOR_DELIVERY"],
  OUT_FOR_DELIVERY: ["DELIVERED", "FAILED"],
};

export function legalNext(status: OrderStatus): OrderStatus[] {
  return NEXT_LEGAL[status] ?? [];
}

export function canTransition(from: OrderStatus, to: OrderStatus): boolean {
  return legalNext(from).includes(to);
}

export function mainFlowIndex(status: OrderStatus): number {
  return MAIN_FLOW.indexOf(status);
}

export function statusLabel(status: OrderStatus): string {
  return LABELS[status];
}

export function allMainFlow(): OrderStatus[] {
  return [...MAIN_FLOW];
}

export function formatINR(amount: number): string {
  return `₹ ${amount.toFixed(2)}`;
}
