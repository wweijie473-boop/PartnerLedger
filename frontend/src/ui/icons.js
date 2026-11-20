import {
  Activity, ArrowLeft, ArrowRight, ArrowUpRight, Box, BriefcaseBusiness, Building2, Check, ChevronDown,
  ChevronRight, CircleAlert, ClipboardList, Clock3, FileText, Gauge, Info, Layers3, LoaderCircle, LogOut,
  PackageCheck, Pause, Play, Plus, Search, ShieldAlert, Store, Truck, Workflow, Menu, X,
} from 'lucide-vue-next'

// Use the same name for the same meaning across pages.
export const icons = Object.freeze({
  workflow: Workflow, overview: Gauge, catalog: Layers3, request: ClipboardList,
  quote: FileText, order: PackageCheck, delivery: Truck, product: Box,
  dealer: Store, vendor: Building2, customer: BriefcaseBusiness, activity: Activity,
  add: Plus, search: Search, back: ArrowLeft, forward: ArrowRight,
  menu: Menu, close: X, pause: Pause, play: Play,
  'open-detail': ArrowUpRight, 'chevron-right': ChevronRight, 'chevron-down': ChevronDown,
  'sign-out': LogOut, check: Check, time: Clock3, info: Info, error: CircleAlert, permission: ShieldAlert, loading: LoaderCircle,
})
