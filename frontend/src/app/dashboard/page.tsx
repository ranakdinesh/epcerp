"use client";
import {NotificationBell} from "@/components/dashboard/notification-bell";
import {HolidayCalendar} from "@/components/work-calendar/holiday-calendar";

import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { EnvelopeIcon, HomeIcon, FolderIcon, BuildingOfficeIcon, TagIcon, GlobeAsiaAustraliaIcon, MapIcon, MapPinIcon, UsersIcon, ShieldCheckIcon, Bars3Icon, ChevronDoubleLeftIcon, ChevronDoubleRightIcon, XMarkIcon, SunIcon, MoonIcon, ChevronDownIcon } from "@heroicons/react/24/outline";
import { PersonalAccountDialog, UserIdentity } from "@/components/personal-profile";
import dynamic from "next/dynamic";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import "./go-workspace.css";
import { ProcessNavigation } from "@/components/processes/process-navigation";
import { processForScreen, availableProcessStep } from "@/components/processes/process-directory";

const loading = () => <p role="status">Loading workspace…</p>;
const ProcurementDashboard = dynamic(()=>import("@/components/dashboard/procurement-dashboard").then(m=>m.ProcurementDashboard),{loading});
const PersonalTasks = dynamic(()=>import("@/components/tasks/personal-tasks").then(m=>m.PersonalTasks),{loading});
const AdvanceRequests = dynamic(()=>import("@/components/purchase-fms/advance-requests").then(m=>m.AdvanceRequests),{loading});
const QuotationRequests = dynamic(()=>import("@/components/purchasing/quotation-requests").then(m=>m.QuotationRequests),{loading});
const PurchaseFMS = dynamic(() => import("@/components/purchase-fms/purchase-fms").then(m => m.PurchaseFMS), { loading });
const ProcessMap = dynamic(() => import("@/components/processes/process-map").then(m => m.ProcessMap), { loading });
const Projects = dynamic(() => import("@/components/projects/project-ownership-workspace").then(m => m.ProjectOwnershipWorkspace), { loading });
const Clients = dynamic(() => import("@/components/clients/client-management").then(m => m.ClientManagement), { loading });
const Categories = dynamic(() => import("@/components/items/category-directory").then(m => m.CategoryDirectory), { loading });
const TenderFMS = dynamic(() => import("@/components/tenders/tender-document-workspace").then(m => m.TenderDocumentWorkspace), { loading });
const Items = dynamic(() => import("@/components/items/item-bank").then(m => m.ItemBank), { loading });
const Manufacturers = dynamic(() => import("@/components/items/manufacturer-bank").then(m=>m.ManufacturerBank), {loading});
const Suppliers = dynamic(() => import("@/components/items/supplier-directory").then(m => m.SupplierDirectory), { loading });
const References = dynamic(() => import("@/components/clients/client-reference-management").then(m => m.ClientReferenceManagement), { loading });
const Offboarding = dynamic(()=>import("@/components/users/offboarding-management").then(m=>m.OffboardingManagement),{loading});
const Organisation = dynamic(()=>import("@/components/users/organisation-management").then(m=>m.OrganisationManagement),{loading});
const Users = dynamic(() => import("@/components/users/user-management").then(m => m.UserManagement), { loading });
const Inbox = dynamic(()=>import("@/components/communication/personal-inbox").then(m=>m.PersonalInbox),{loading});
const Director = dynamic(()=>import("@/components/organisation-governance/director-designation").then(m=>m.DirectorDesignation),{loading});
const Roles = dynamic(() => import("@/components/roles/role-management").then(m => m.RoleManagement), { loading });

const navigation = [
 {screen:"my-team",label:"My team",group:"Communication",permission:"account.team.read"},
 {screen:"my-tasks",label:"My tasks",group:"Communication",permission:"work.tasks.view"},
 {screen:"purchase-advances",label:"Advance requests",group:"Accounts",permission:"accounts.purchase_advances.view"},
 {screen:"quotation-approvals",label:"Quotation approvals",group:"Accounts",permission:"accounts.quotation_approvals.view"},
 {screen:"inbox",label:"My inbox",group:"Communication",permission:"communication.inbox.read"},
 {screen:"director",label:"Director designation",group:"Administration",permission:"admin.governance.read"},
 {screen:"organisation",label:"Organisation",group:"Administration",permission:"admin.organisation.view"},
 {screen:"offboarding",label:"Handovers & deactivation",group:"Administration",permission:"account.offboarding.read"},
 {screen:"holidays",label:"Holiday calendar",group:"Global masters",permission:"settings.holidays.view"},
  { screen: "purchase-fms", label: "Purchase FMS", group: "Project delivery", permission: "projects.purchase_workflow.view" },
  { screen: "tender-fms", label: "Tender FMS", group: "Project delivery", permission: "tenders.register.view" },
  { screen: "projects", label: "Projects", group: "Project delivery", permission: "projects.master.view" },
  { screen: "clients", label: "Clients", group: "Global masters", permission: "settings.clients.view" },
  { screen: "client-types", label: "Client types", group: "Global masters", permission: "settings.client_types.view" },
  { screen: "suppliers", label: "Vendors / Suppliers", group: "Global masters", permission: "settings.suppliers.view" },
  { screen: "items", label: "Item bank", group: "Global masters", permission: "settings.items.view" },
  { screen: "manufacturers", label: "Manufacturers", group: "Global masters", permission: "settings.manufacturers.view" },
  { screen: "categories", label: "Categories", group: "Global masters", permission: "settings.categories.view" },
  { screen: "industries", label: "Industries", group: "Global masters", permission: "settings.industries.view" },
  { screen: "states", label: "States", group: "Global masters", permission: "settings.locations.view" },
  { screen: "cities", label: "Cities", group: "Global masters", permission: "settings.locations.view" },
  { screen: "users", label: "Users", group: "Administration", permission: "admin.users.view" },
  { screen: "roles", label: "Roles & permissions", group: "Administration", permission: "admin.roles.view" },
] as const;
const icons = {offboarding:UsersIcon,organisation:UsersIcon,"my-team":UsersIcon,"my-tasks":FolderIcon,"purchase-advances":ShieldCheckIcon, "quotation-approvals":ShieldCheckIcon, inbox:EnvelopeIcon, director:ShieldCheckIcon, holidays:MapIcon, "purchase-fms": FolderIcon, "tender-fms": FolderIcon, items: TagIcon, manufacturers: BuildingOfficeIcon, categories: TagIcon, projects: FolderIcon, clients: BuildingOfficeIcon, suppliers: BuildingOfficeIcon, "client-types": TagIcon, industries: GlobeAsiaAustraliaIcon, states: MapIcon, cities: MapPinIcon, users: UsersIcon, roles: ShieldCheckIcon };
function subscribePreference(cb: () => void) { window.addEventListener("epcerp-preference", cb); window.addEventListener("storage", cb); return () => { window.removeEventListener("epcerp-preference", cb); window.removeEventListener("storage", cb); }; }
function preference(key: string) { try { return localStorage.getItem(key); } catch { return null; } }
function savePreference(key: string, value: string) { try { localStorage.setItem(key, value); } catch {} window.dispatchEvent(new Event("epcerp-preference")); }


export default function DashboardPage() {
  const user = useAuthenticatedUser();
  const [account, setAccount] = useState<"profile" | "password" | null>(null);
  const mobileNav = useRef<HTMLDialogElement>(null);
  const accountMenu = useRef<HTMLDetailsElement>(null);
  const collapsed = useSyncExternalStore(subscribePreference, () => preference("epcerp-sidebar") === "collapsed", () => false);
  const dark = useSyncExternalStore(subscribePreference, () => document.documentElement.dataset.theme === "dark", () => false);
  function toggleTheme() { const theme = dark ? "light" : "dark"; document.documentElement.dataset.theme = theme; savePreference("epcerp-theme", theme); }
  useEffect(() => { function dismiss(e: MouseEvent | KeyboardEvent) { if (e instanceof KeyboardEvent ? e.key === "Escape" : !accountMenu.current?.contains(e.target as Node)) accountMenu.current?.removeAttribute("open"); } document.addEventListener("click", dismiss); document.addEventListener("keydown", dismiss); return () => { document.removeEventListener("click", dismiss); document.removeEventListener("keydown", dismiss); }; }, []);
  const params = useSearchParams();
  const router = useRouter();
  const permissions = new Set(user.permissions);
  const visible = navigation.filter(item => permissions.has("system:admin") || (permissions.has(item.permission) || (item.screen === "suppliers" && permissions.has("accounts.vendor_verification.view"))));
  const selected = visible.find(item => item.screen === params.get("screen"));
  const screen = selected?.screen ?? "overview";
  const process = screen === "purchase-fms" && !params.get("project") ? undefined : processForScreen(screen);
  const mapSelected = Boolean(process && (params.get("view") === "map"));
  const workPackages = process?.screen === "purchase-fms" && params.get("work") === "packages";
  const processStep = process ? availableProcessStep(process, params.get("step")) : undefined;
  async function logout() {
    const response = await fetch("/api/auth/logout", { method: "POST" });
    // eslint-disable-next-line @next/next/no-location-assign-relative-destination -- discard authenticated router state after cookies are cleared
    if (response.ok) window.location.assign("/login?session=signed-out");
  }
  function navLinks(mobile = false) {
    if (process) return <ProcessNavigation key={process.screen} process={process} selected={processStep} mapSelected={mapSelected} workPackages={workPackages} projectId={params.get("project") ?? undefined} draftId={params.get("draft") ?? undefined} close={() => { if (mobile) mobileNav.current?.close(); }} />;
    return <>
    <Link href="/dashboard" title="Workspace" aria-label="Workspace" aria-current={screen === "overview" ? "page" : undefined} onClick={() => { if (mobile) mobileNav.current?.close(); }}><HomeIcon aria-hidden="true" /><span>Workspace</span></Link>
    {[...new Set(visible.map(item => item.group))].map(group => <div key={group}><p>{group}</p>{visible.filter(item => item.group === group).map(item => { const Icon=icons[item.screen]; return <Link key={item.screen} href={`/dashboard?screen=${item.screen}`} title={item.label} aria-label={item.label} aria-current={screen === item.screen ? "page" : undefined} onClick={() => { if (mobile) mobileNav.current?.close(); }}><Icon aria-hidden="true" /><span>{item.label}</span></Link>; })}</div>)}
  </>; }
  return (
    <div className={`go-workspace${process ? " go-process-workspace" : ""}${collapsed ? " go-collapsed" : ""}`}>
      <a href="#workspace-content" className="go-skip-link">Skip to workspace</a>
      <header className="go-appbar">
        <div className="go-brand-group"><button type="button" className="go-icon-button go-mobile-toggle" aria-label="Open navigation" aria-haspopup="dialog" onClick={() => mobileNav.current?.showModal()}><Bars3Icon /></button><Link href="/dashboard" className="go-brand">VTPL <strong>CORE</strong><span>epcERP</span></Link></div>
        <div className="go-account"><NotificationBell/><button type="button" className="go-icon-button" title={dark ? "Switch to light mode" : "Switch to dark mode"} aria-label={dark ? "Switch to light mode" : "Switch to dark mode"} onClick={toggleTheme}>{dark ? <SunIcon /> : <MoonIcon />}</button><details ref={accountMenu} className="go-account-menu"><summary aria-label="Open Me menu"><UserIdentity id={user.id} name={user.username} /><ChevronDownIcon aria-hidden="true" /></summary><div className="go-account-dropdown"><p>My account</p><button type="button" onClick={() => { accountMenu.current?.removeAttribute("open"); setAccount("profile"); }}>Me · Edit profile</button><button type="button" onClick={() => { accountMenu.current?.removeAttribute("open"); setAccount("password"); }}>Change password</button><hr /><button type="button" onClick={() => void logout()}>Sign out</button></div></details></div>
      </header>
      <nav id="desktop-navigation" aria-label={process ? `${process.label} process navigation` : "Primary navigation"} className="go-navigation go-desktop-navigation"><button type="button" className="go-icon-button go-desktop-toggle" title={collapsed ? "Expand navigation" : "Collapse navigation"} aria-label={collapsed ? "Expand navigation" : "Collapse navigation"} aria-expanded={!collapsed} aria-controls="desktop-navigation" onClick={() => savePreference("epcerp-sidebar", collapsed ? "expanded" : "collapsed")}>{collapsed ? <ChevronDoubleRightIcon /> : <ChevronDoubleLeftIcon />}</button>{navLinks()}</nav>
      <dialog ref={mobileNav} className="go-mobile-navigation" aria-label="Navigation" onClick={e => { if (e.target === e.currentTarget && e.clientX > e.currentTarget.getBoundingClientRect().right) mobileNav.current?.close(); }}><div className="go-dialog-header"><strong>{process?.label ?? "Workspace"}</strong><button type="button" aria-label="Close navigation" className="go-icon-button" onClick={() => mobileNav.current?.close()}><XMarkIcon /></button></div><nav aria-label={process ? `${process.label} mobile process navigation` : "Mobile navigation"} className="go-navigation">{navLinks(true)}</nav></dialog>
      {account && <PersonalAccountDialog key={account} mode={account} close={() => { setAccount(null); accountMenu.current?.querySelector("summary")?.focus(); }} />}
      <main id="workspace-content" className="go-content">
        {process && <nav aria-label="Breadcrumb" className="go-process-breadcrumb"><Link href="/dashboard">Workspace</Link><span aria-hidden="true">/</span><span>{process.label}</span><span aria-hidden="true">/</span><span aria-current="page">{mapSelected ? "Process map" : workPackages ? "Purchase work packages" : processStep?.label}</span></nav>}
        {screen === "my-tasks" ? <PersonalTasks/> : screen === "purchase-advances" ? <AdvanceRequests/> : screen === "quotation-approvals" ? <QuotationRequests/> : screen === "purchase-fms" ? <PurchaseFMS allowedScreens={visible.map(item=>item.screen)} /> : process && mapSelected ? <ProcessMap key={process.screen} process={process} allowedScreens={visible.map(item=>item.screen)} /> : screen === "overview" ? (permissions.has("system:admin")||(permissions.has("projects.purchase_workflow.view")&&permissions.has("projects.purchase_workflow.read"))) ? <ProcurementDashboard/> : <>
          <div className="go-page-heading"><p className="go-eyebrow">Project execution workspace</p><h1>Work, decisions and follow-through.</h1></div>
          <section className="go-available" aria-labelledby="available-heading"><h2 id="available-heading">Available workspaces</h2>
            {visible.length ? <ul>{visible.map(item => <li key={item.screen}><Link href={`/dashboard?screen=${item.screen}`}>{item.label}<span aria-hidden="true"> →</span></Link><span>{item.group}</span></li>)}</ul> : <p>Your account has no directory workspace access. Contact your administrator.</p>}
          </section>
          {permissions.has("projects.budgets.view")&&<p className="text-sm text-slate-600">Open <Link href="/dashboard?screen=projects" className="text-blue-700 underline">Projects</Link> to review budgets within your assigned Finance responsibility. Supplier reviews appear in My tasks; submitted quotations appear in Quotation approvals.</p>}
        </> : screen === "projects" ? <Projects />
          : screen === "clients" ? <Clients />
          : screen === "holidays" ? <HolidayCalendar/>
          : screen === "suppliers" ? <Suppliers />
          : screen === "tender-fms" ? <TenderFMS step={processStep?.code} draftID={params.get("draft") ?? undefined} onSelectStep={step=>{const next=new URLSearchParams(params.toString());next.set("step",step);router.replace(`/dashboard?${next.toString()}`,{scroll:false});}} onSelectDraft={id=>{const next=new URLSearchParams(params.toString());if(id)next.set("draft",id);else next.delete("draft");router.replace(`/dashboard?${next.toString()}`,{scroll:false});}} />
          : screen === "items" ? <Items />
          : screen === "manufacturers" ? <Manufacturers />
          : screen === "categories" ? <Categories />
          : screen === "offboarding" ? <Offboarding />
          : screen === "organisation" ? <Organisation />
          : screen === "my-team" ? <Organisation team />
          : screen === "users" ? <Users />
          : screen === "inbox" ? <Inbox /> : screen === "director" ? <Director />
          : screen === "roles" ? <Roles />
          : screen === "client-types" || screen === "industries" || screen === "states" || screen === "cities" ? <References key={screen} kind={screen} /> : null}
      </main>
    </div>
  );
}
