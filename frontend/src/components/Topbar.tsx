import { Minus, X } from "lucide-react";
import { QuitApp, MinimiseApp } from "../../bindings/AuOvIP/app";

function Topbar() {
    return (
        <div className='flex flex-row items-center justify-between px-4 h-10 select-none bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/60 border-b border-border z-50'>
            {/* Draggable Area */}
            <div id="topbarDraggablePart" className='flex-1 h-full flex items-center'>
                <p className='text-xs font-semibold text-muted-foreground tracking-wide'>AUDIO OVER IP</p>
            </div>
            
            {/* Window Controls */}
            <div className='flex items-center space-x-2'>
                <button 
                    className='p-1.5 hover:bg-accent hover:text-accent-foreground rounded-sm transition-colors focus:outline-none focus:ring-1 focus:ring-ring' 
                    onClick={MinimiseApp}
                    aria-label="Minimize"
                >
                    <Minus className="w-4 h-4"/>
                </button>
                <button 
                    className='p-1.5 hover:bg-destructive hover:text-destructive-foreground rounded-sm transition-colors focus:outline-none focus:ring-1 focus:ring-destructive' 
                    onClick={QuitApp}
                    aria-label="Close"
                >
                    <X className="w-4 h-4"/>
                </button>
            </div>
        </div>
    )
}

export default Topbar