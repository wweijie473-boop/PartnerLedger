import {
  Activity, ArrowLeft, ArrowRight, ArrowUp, ArrowUpRight, Box, BriefcaseBusiness, Building2, Check, ChevronDown,
  ChevronRight, CircleAlert, ClipboardList, Clock3, FileText, Folder, Gauge, Info, Layers3, LoaderCircle, LogOut,
  MessageSquare, Mic, Moon, PackageCheck, PanelLeft, PanelLeftClose, Pause, Play, Plus, Search, ShieldAlert,
  ShieldCheck, Square, SquarePen, Store, Sun, Trash2, Truck, Workflow, Menu, X,
} from 'lucide-vue-next'
import UiAiIcon from './UiAiIcon.vue'

// Use the same name for the same meaning across pages.
export const icons = Object.freeze({
  ai: UiAiIcon,
  workflow: Workflow, overview: Gauge, catalog: Layers3, request: ClipboardList,
  quote: FileText, order: PackageCheck, delivery: Truck, product: Box,
  dealer: Store, vendor: Building2, customer: BriefcaseBusiness, activity: Activity,
  add: Plus, search: Search, back: ArrowLeft, forward: ArrowRight, 'send-up': ArrowUp,
  folder: Folder, 'new-chat': SquarePen, chat: MessageSquare,
  'sidebar-open': PanelLeft, 'sidebar-close': PanelLeftClose, trash: Trash2,
  moon: Moon, sun: Sun, 'shield-check': ShieldCheck,
  menu: Menu, close: X, pause: Pause, play: Play, mic: Mic, stop: Square,
  'open-detail': ArrowUpRight, 'chevron-right': ChevronRight, 'chevron-down': ChevronDown,
  'sign-out': LogOut, check: Check, time: Clock3, info: Info, error: CircleAlert, permission: ShieldAlert, loading: LoaderCircle,
})
