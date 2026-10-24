"use client";
import {ProcurementDashboard} from "@/components/dashboard/procurement-dashboard";
// All users share the same dashboard; the API owns portfolio/assignment scope.
export function PurchaseDashboard(){return <ProcurementDashboard purchaseEntry/>;}
