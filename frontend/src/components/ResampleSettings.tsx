import * as Select from "@radix-ui/react-select";
import * as Tooltip from "@radix-ui/react-tooltip";
import { Check, ChevronDown } from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { Options } from "../../bindings/AuOvIP/pcmresample/models";
import { LabelWithTooltip } from "./LabelWithTooltip";

interface ResampleSettingsProps {
    opts: Options;
    onChange: (opts: Options) => void;
    disabled?: boolean;
}

export function ResampleSettings({ opts, onChange, disabled = false }: ResampleSettingsProps) {
    const method = opts.Method;
    const quality = opts.Quality;
    const lfe = opts.IncludeLFEInDownmix;
    const maxQual = 100;

    const setMethod = (m: number) => onChange({ ...opts, Method: m });
    const setQuality = (q: number) => onChange({ ...opts, Quality: q });
    const setLfe = (l: boolean) => onChange({ ...opts, IncludeLFEInDownmix: l });

    return (
        <Tooltip.Provider delayDuration={300}>
            <div className="grid gap-6">
                {/* Method Selection */}
                <div className="grid grid-cols-4 items-center gap-4">
                    <LabelWithTooltip
                        label="Algorithm"
                        tooltip="The interpolation algorithm used to resize the audio stream. 'Sinc' offers the best fidelity but uses more CPU. 'Linear' is the fastest."
                    />
                    <Select.Root
                        value={method.toString()}
                        onValueChange={(v) => setMethod(parseInt(v))}
                        disabled={disabled}
                    >
                        <Select.Trigger className="col-span-3 flex h-9 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50 text-foreground transition-colors">
                            <Select.Value placeholder="Select method" />
                            <Select.Icon>
                                <ChevronDown className="h-4 w-4 opacity-50" />
                            </Select.Icon>
                        </Select.Trigger>
                        <Select.Portal>
                            <Select.Content className="relative z-[110] min-w-[8rem] overflow-hidden rounded-md border border-border bg-popover text-popover-foreground shadow-xl animate-in fade-in-0 zoom-in-95 duration-100">
                                <Select.Viewport className="p-1">
                                    <SelectItem value="0">Linear (Fastest)</SelectItem>
                                    <SelectItem value="1">Cubic (Balanced)</SelectItem>
                                    <SelectItem value="2">Sinc (Best Quality)</SelectItem>
                                </Select.Viewport>
                            </Select.Content>
                        </Select.Portal>
                    </Select.Root>
                </div>

                {/* Quality Input */}
                <div className="grid grid-cols-4 items-center gap-4">
                    <LabelWithTooltip label="Quality" tooltip={GetQualityTooltip(method)} />
                    <div className="col-span-3 flex items-center space-x-3">
                        <input
                            type="number"
                            min={1}
                            max={100}
                            value={quality}
                            onChange={(e) => {
                                const val = parseInt(e.target.value);
                                if (val <= maxQual || Number.isNaN(val)) setQuality(val);
                            }}
                            disabled={disabled}
                            className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 font-mono"
                        />
                        <span className="text-xs font-medium text-muted-foreground/70 bg-muted px-2 py-1 rounded whitespace-nowrap h-9 text-center flex items-center">
                            1-100
                        </span>
                    </div>
                </div>

                {/* LFE Switch */}
                <div className="grid grid-cols-4 items-center gap-4">
                    <LabelWithTooltip
                        label="LFE Mix"
                        tooltip="If enabled, the Low-Frequency Effects (Subwoofer) channel is mixed into the output when downmixing (e.g. 5.1 to Stereo) instead of being discarded."
                    />
                    <div className="col-span-3 flex items-center space-x-3">
                        <Switch
                            checked={lfe}
                            onCheckedChange={setLfe}
                            disabled={disabled}
                            className="data-[state=checked]:bg-green-600"
                        />
                        <span className="text-xs text-muted-foreground italic">Include Low Frequency Effects</span>
                    </div>
                </div>
            </div>
        </Tooltip.Provider>
    );
}

const GetQualityTooltip = (m: number) => {
    switch (m) {
        case 0: return "Linear interpolation is the fastest method. The Quality setting only slightly adjusts the output gain to prevent clipping.";
        case 1: return `Controls the sharpness of the cubic spline. Higher values result in sharper audio.`;
        case 2: return `Controls the window size (taps) of the Sinc filter. Higher values provide better anti-aliasing but SIGNIFICANTLY increase CPU and RAM usage. Values >20 not recommended.`;
        default: return "";
    }
};

const SelectItem = ({ children, value, ...props }: any) => {
    return (
        <Select.Item
            value={value}
            className={cn(
                "relative flex w-full cursor-default select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-sm outline-none focus:bg-accent focus:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50 transition-colors",
            )}
            {...props}
        >
            <span className="absolute left-2 flex h-3.5 w-3.5 items-center justify-center">
                <Select.ItemIndicator>
                    <Check className="h-4 w-4" />
                </Select.ItemIndicator>
            </span>
            <Select.ItemText>{children}</Select.ItemText>
        </Select.Item>
    );
};
