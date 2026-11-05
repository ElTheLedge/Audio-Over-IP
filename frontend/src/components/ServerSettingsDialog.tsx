import { useState, useEffect } from "react";
import * as Dialog from "@radix-ui/react-dialog";
import * as Select from "@radix-ui/react-select";
import * as Tooltip from "@radix-ui/react-tooltip";
import { Settings, Check, ChevronDown, Loader2, X, Trash2, Volume2, VolumeX, Speaker } from "lucide-react";
import { Server, PlaybackDevice } from "../../bindings/AuOvIP/models";
import { Options } from "../../bindings/AuOvIP/pcmresample/models";
import { AudioConfig } from "../../bindings/AuOvIP/wcatools/models";
import { UpdateServerSettings, ListAudioDevices, AddPlaybackDevice, RemovePlaybackDevice, UpdatePlaybackDevice, SetServerRemoteDevice } from "../../bindings/AuOvIP/app";
import { Button } from "./ui/button";
import { Switch } from "./ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./ui/tabs";
import { Slider } from "./ui/slider";
import { cn } from "../lib/utils";
import { Events } from "@wailsio/runtime";
import { ResampleSettings } from "./ResampleSettings";
import { LabelWithTooltip } from "./LabelWithTooltip";

interface ServerSettingsDialogProps {
    server: Server;
}

export function ServerSettingsDialog({ server }: ServerSettingsDialogProps) {
    const [open, setOpen] = useState(false);
    const [isLoading, setIsLoading] = useState(false);

    // Resampling State
    const [resampleOpts, setResampleOpts] = useState<Options>(server.ResampleOpts || {
        Method: 2, Quality: 20, IncludeLFEInDownmix: true, MaxQuality: 100, MaxThreads: 0, CustomMixMatrix: null
    });

    // Devices State
    const [availableDevices, setAvailableDevices] = useState<AudioConfig[]>([]);
    const [selectedDeviceToAdd, setSelectedDeviceToAdd] = useState<string>("");

    useEffect(() => {
        if (open) {
            fetchDevices();
            // Subscribe to device changes
            const unsubscribers: Array<() => void> = [];
            unsubscribers.push(Events.On("deviceChanged", () => { fetchDevices() }));
            return () => {
                for (const off of unsubscribers) off();
            };
        }
    }, [open]);

    const fetchDevices = async () => {
        try {
            const devs = await ListAudioDevices();
            setAvailableDevices(devs || []);
        } catch (e) {
            console.error("Failed to list devices", e);
        }
    };

    const handleSaveResampling = async () => {
        setIsLoading(true);
        try {
            await UpdateServerSettings(server.ID, resampleOpts);
        } catch (e) {
            console.error("Failed to update settings", e);
        } finally {
            setIsLoading(false);
        }
    };

    const handleAddDevice = async () => {
        if (!selectedDeviceToAdd) return;
        try {
            await AddPlaybackDevice(server.ID, selectedDeviceToAdd);
            setSelectedDeviceToAdd("");
        } catch (e) {
            console.error(e);
        }
    };



    // Filter available devices to exclude already added ones
    const unaddedDevices = availableDevices.filter(d =>
        !server.PlaybackDevices?.some(pd => pd.deviceID === d.ID)
    );

    return (
        <Dialog.Root open={open} onOpenChange={(o) => {
            if (isLoading) return;
            if (o) {
                setResampleOpts(server.ResampleOpts || {
                    Method: 2, Quality: 20, IncludeLFEInDownmix: true, MaxQuality: 100, MaxThreads: 0, CustomMixMatrix: null
                });
            }
            setOpen(o);
        }}>
            <Dialog.Trigger asChild>
                <Button disabled={!server.Online} variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-foreground">
                    <Settings className="h-4 w-4" />
                </Button>
            </Dialog.Trigger>
            <Dialog.Portal>
                <Dialog.Overlay className="fixed inset-0 bg-black/60 backdrop-blur-sm z-[100] animate-in fade-in-0 duration-200" />
                <Dialog.Content
                    onInteractOutside={(e) => { if (isLoading) e.preventDefault(); }}
                    onEscapeKeyDown={(e) => { if (isLoading) e.preventDefault(); }}
                    className="fixed left-[50%] top-[50%] z-[101] grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-6 border border-border bg-card p-6 shadow-2xl duration-200 animate-in fade-in-0 zoom-in-95 sm:rounded-xl"
                >
                    <div className="flex flex-col space-y-1.5 text-left">
                        <Dialog.Title className="text-xl font-bold tracking-tight text-foreground">
                            Server Settings
                        </Dialog.Title>
                        <Dialog.Description className="text-sm text-muted-foreground">
                            Configuration for <span className="font-mono text-primary">{server.Addr}</span>
                        </Dialog.Description>
                    </div>

                    <Tooltip.Provider delayDuration={300}>
                        <Tabs defaultValue="devices" className="w-full">
                            <TabsList className="grid w-full grid-cols-2 mb-4">
                                <TabsTrigger value="devices">Device Selection</TabsTrigger>
                                <TabsTrigger value="resampling">Resampling</TabsTrigger>
                            </TabsList>

                            <TabsContent value="devices" className="space-y-4 max-h-[400px] overflow-y-auto pr-2">
                                {/* Remote Device Selection */}
                                <div className="space-y-2 border-b pb-4">
                                    <LabelWithTooltip position="left" label="Remote Capture Device" tooltip="Select which audio device on the server to capture audio from." />
                                    <Select.Root value={server.RemoteDeviceID || "default"} onValueChange={(val) => SetServerRemoteDevice(server.ID, val)}>
                                        <Select.Trigger className="flex min-h-9 h-fit w-full items-center justify-between rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring text-foreground">
                                            <Select.Value placeholder="Select remote device" />
                                            <Select.Icon>
                                                <ChevronDown className="h-4 w-4 opacity-50" />
                                            </Select.Icon>
                                        </Select.Trigger>
                                        <Select.Portal>
                                            <Select.Content className="relative z-[110] min-w-[8rem] overflow-hidden rounded-md border border-border bg-popover text-popover-foreground shadow-xl animate-in fade-in-0 zoom-in-95 duration-100 max-h-[300px]">
                                                <Select.Viewport className="p-1">
                                                    <SelectItem value="default">Default Output Device</SelectItem>
                                                    {server.RemoteDevices?.map((d) => (
                                                        <SelectItem key={d.ID} value={d.ID}>{d.Name}</SelectItem>
                                                    ))}
                                                    {server.RemoteDeviceID && server.RemoteDeviceID !== "default" && !server.RemoteDevices?.some(d => d.ID === server.RemoteDeviceID) && (
                                                        <SelectItem value={server.RemoteDeviceID} disabled>
                                                            Unavailable Device
                                                        </SelectItem>
                                                    )}
                                                </Select.Viewport>
                                            </Select.Content>
                                        </Select.Portal>
                                    </Select.Root>
                                </div>

                                {/* Device List */}
                                <div className="space-y-3">
                                    <LabelWithTooltip position="left" label="Playback Devices" tooltip="Select on which audio devices to play the stream" />
                                    {server.PlaybackDevices?.map((device) => {
                                        const isAvailable = availableDevices.some(d => d.ID === device.deviceID);
                                        return (
                                            <PlaybackDeviceRow
                                                key={device.deviceID}
                                                device={device}
                                                serverID={server.ID}
                                                isAvailable={isAvailable}
                                            />
                                        );
                                    })}
                                    {(!server.PlaybackDevices || server.PlaybackDevices.length === 0) && (
                                        <div className="text-center py-8 text-sm text-muted-foreground border border-dashed rounded-lg">
                                            No playback devices configured.
                                        </div>
                                    )}
                                </div>

                                {/* Add Device */}
                                <div className="flex items-center gap-3">
                                    <Select.Root value={selectedDeviceToAdd} onValueChange={setSelectedDeviceToAdd}>
                                        <Select.Trigger className="flex min-h-9 h-fit flex-1 items-center justify-between rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring text-foreground">
                                            <Select.Value placeholder="Select device to add" />
                                            <Select.Icon>
                                                <ChevronDown className="h-4 w-4 opacity-50" />
                                            </Select.Icon>
                                        </Select.Trigger>
                                        <Select.Portal>
                                            <Select.Content className="relative z-[110] min-w-[8rem] overflow-hidden rounded-md border border-border bg-popover text-popover-foreground shadow-xl animate-in fade-in-0 zoom-in-95 duration-100 max-h-[300px]">
                                                <Select.Viewport className="p-1">
                                                    {unaddedDevices.map((d) => (
                                                        <SelectItem key={d.ID} value={d.ID}>{d.Name}</SelectItem>
                                                    ))}
                                                    {unaddedDevices.length === 0 && (
                                                        <div className="p-2 text-xs text-muted-foreground text-center">No other devices available</div>
                                                    )}
                                                </Select.Viewport>
                                            </Select.Content>
                                        </Select.Portal>
                                    </Select.Root>
                                    <Button onClick={handleAddDevice} disabled={!selectedDeviceToAdd}>Add</Button>
                                </div>
                            </TabsContent>

                            <TabsContent value="resampling">
                                <div className="space-y-6 py-2">
                                    <ResampleSettings
                                        opts={resampleOpts}
                                        onChange={setResampleOpts}
                                        disabled={isLoading}
                                    />
                                    <div className="flex justify-end pt-4">
                                        <Button
                                            onClick={handleSaveResampling}
                                            disabled={isLoading}
                                            className="w-full sm:w-auto"
                                        >
                                            {isLoading ? (
                                                <>
                                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                                    Applying
                                                </>
                                            ) : (
                                                "Apply Resampling Settings"
                                            )}
                                        </Button>
                                    </div>
                                </div>
                            </TabsContent>
                        </Tabs>
                    </Tooltip.Provider>

                    <Dialog.Close asChild>
                        <button
                            disabled={isLoading}
                            className="absolute right-4 top-4 rounded-sm opacity-70 transition-opacity hover:opacity-100 focus:outline-none disabled:pointer-events-none text-foreground"
                        >
                            <X className="h-4 w-4" />
                            <span className="sr-only">Close</span>
                        </button>
                    </Dialog.Close>
                </Dialog.Content>
            </Dialog.Portal>
        </Dialog.Root>
    );
}



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

const PlaybackDeviceRow = ({ device, serverID, isAvailable }: { device: PlaybackDevice, serverID: string, isAvailable: boolean }) => {
    const [localVolume, setLocalVolume] = useState(device.volume);
    const [isLoading, setIsLoading] = useState(false);
    const [isEnabled, setIsEnabled] = useState(device.enabled);

    // If backend update comes in, we want to reflect it.
    useEffect(() => {
        setLocalVolume(device.volume);
    }, [device.volume]);

    useEffect(() => {
        if (!isLoading) {
            setIsEnabled(device.enabled);
        }
    }, [device.enabled, isLoading]);

    const handleVolumeCommit = (val: number) => {
        UpdatePlaybackDevice(serverID, device.deviceID, isEnabled, val);
    };

    const handleToggle = async (c: boolean) => {
        setIsLoading(true);
        setIsEnabled(c); // Optimistic update
        try {
            await UpdatePlaybackDevice(serverID, device.deviceID, c, localVolume);
        } catch (e) {
            // Revert on failure
            setIsEnabled(!c);
            console.error(e);
        } finally {
            setIsLoading(false);
        }
    };

    // Allow for volumes from 0% to 150%
    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        let val = parseInt(e.target.value);
        if (isNaN(val)) val = 0;
        if (val > 150) val = 150;
        if (val < 0) val = 0;

        const floatVal = val / 100;
        setLocalVolume(floatVal);
        UpdatePlaybackDevice(serverID, device.deviceID, isEnabled, floatVal);
    };

    return (
        <div className={cn("flex flex-col gap-3 rounded-lg border p-3 shadow-sm bg-muted/40", !isAvailable && !device.isDefault && "opacity-70 border-red-900/50 bg-red-900/10")}>
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <Speaker className="h-4 w-4 text-primary" />
                    <div className="flex flex-col">
                        <span className="text-sm font-medium">
                            {device.name}
                            {device.isDefault && <span className="ml-2 text-[10px] bg-primary/20 text-primary px-1.5 py-0.5 rounded-full">System Default</span>}
                            {!isAvailable && !device.isDefault && <span className="ml-2 text-[10px] bg-red-500/20 text-red-500 px-1.5 py-0.5 rounded-full">Unavailable</span>}
                        </span>
                    </div>
                </div>
                <div className="flex items-center gap-2">
                    <Switch
                        checked={isEnabled}
                        onCheckedChange={handleToggle}
                        disabled={isLoading}
                        className="data-[state=checked]:bg-green-600"
                    />
                    <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 text-muted-foreground hover:text-destructive"
                        onClick={() => RemovePlaybackDevice(serverID, device.deviceID)}
                    >
                        <Trash2 className="h-4 w-4" />
                    </Button>
                </div>
            </div>

            {/* Volume Control */}
            <div className="flex items-center gap-4 px-1">
                {localVolume === 0 ? <VolumeX className="h-4 w-4 text-muted-foreground" /> : <Volume2 className="h-4 w-4 text-muted-foreground" />}

                <Slider
                    value={[localVolume]}
                    max={1.5}
                    step={0.01}
                    onValueChange={(v) => setLocalVolume(v[0])}
                    onValueCommit={(v) => handleVolumeCommit(v[0])}
                    className="flex-1 cursor-grab active:cursor-grabbing"
                />

                <div className="flex items-center">
                    <input
                        type="number"
                        min={0}
                        max={150}
                        value={Math.round(localVolume * 100)}
                        onChange={handleInputChange}
                        className="w-12 h-8 rounded-md border border-input bg-background px-2 py-1 text-xs text-right shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    />
                    <span className="ml-1 text-xs text-muted-foreground">%</span>
                </div>
            </div>
        </div>
    );
};