import { useEffect, useState } from "react";
import { AddServer, RemoveServer, ConnectToAudioServer, DisconnetFromAudioServer, GetServerList, ReorderServers, CheckServerConnection } from "../../bindings/AuOvIP/app";
import { Events } from "@wailsio/runtime";
import { Server } from "../../bindings/AuOvIP/models";
import { ServerSettingsDialog } from "../components/ServerSettingsDialog";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Plus, Trash2, Wifi, WifiOff, GripVertical, Loader2, RefreshCw } from "lucide-react";
import { useToast } from "@/components/ui/use-toast";
import { DndContext, closestCenter, KeyboardSensor, PointerSensor, useSensor, useSensors, DragEndEvent } from "@dnd-kit/core";
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, rectSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";

function ClientPage() {
    const [ipAddress, setIpAddress] = useState("");
    const [port, setPort] = useState("");
    const [servers, setServers] = useState<Server[]>([]);
    const [isAdding, setIsAdding] = useState(false);
    const { toast } = useToast();

    const sensors = useSensors(
        useSensor(PointerSensor),
        useSensor(KeyboardSensor, {
            coordinateGetter: sortableKeyboardCoordinates,
        })
    );

    useEffect(() => {
        GetServerList().then((list) => {
            setServers(list || []);
        });

        const updateHandler = (e: Events.WailsEvent<"updateConnStatus">) => {
            const data = e.data;
            setServers((prev) => prev.map((s) => (s.ID === data.serverId) ? { ...s, Status: data.statusText } : s));
        };

        const listHandler = (e: Events.WailsEvent<"serversUpdated">) => {
            setServers(e.data || []);
        };

        const unsubscribers: Array<() => void> = [];
        unsubscribers.push(Events.On("updateConnStatus", updateHandler));
        unsubscribers.push(Events.On("serversUpdated", listHandler));

        return () => {
            for (const off of unsubscribers) off();
        };
    }, []);

    const handleAdd = async () => {
        if (!ipAddress || !port) return;
        setIsAdding(true);
        try {
            await AddServer(ipAddress, port);
            setIpAddress("");
            setPort("");
        } catch (e: any) {
            toast({
                variant: "destructive",
                title: "Connection Failed",
                description: e || "Could not connect to server.",
            });
        } finally {
            setIsAdding(false);
        }
    };

    const handleDragEnd = (event: DragEndEvent) => {
        const { active, over } = event;

        if (over && active.id !== over.id) {
            setServers((items) => {
                const oldIndex = items.findIndex((item) => item.ID === active.id);
                const newIndex = items.findIndex((item) => item.ID === over.id);
                const newOrder = arrayMove(items, oldIndex, newIndex);

                ReorderServers(newOrder.map(s => s.ID));

                return newOrder;
            });
        }
    };

    return (
        <div className="flex flex-col space-y-8 h-full w-full">
            {/* Header / Add Section */}
            <div className="flex flex-col space-y-4">
                <div className="flex items-center justify-between">
                    <div>
                        <h1 className="text-2xl font-bold tracking-tight">Receiver Mode</h1>
                        <p className="text-muted-foreground">Manage your inbound audio connections.</p>
                    </div>
                </div>

                <Card className="bg-card/50">
                    <CardContent className="p-4 flex flex-col gap-2">
                        <div className="flex items-center gap-4">
                            <Input
                                placeholder="IP Address (e.g. 192.168.1.5)"
                                className="flex-1 font-mono"
                                value={ipAddress}
                                onChange={(e) => setIpAddress(e.target.value)}
                                disabled={isAdding}
                            />
                            <div className="w-px h-8 bg-border" />
                            <Input
                                placeholder="Port"
                                type="number"
                                className="w-32 font-mono"
                                value={port}
                                onChange={(e) => setPort(e.target.value)}
                                disabled={isAdding}
                            />
                            <Button onClick={handleAdd} size="sm" className="ml-2 w-32" disabled={isAdding}>
                                {isAdding ? <Loader2 className="w-4 h-4 animate-spin" /> : <><Plus className="w-4 h-4 mr-2" /> Add</>}
                            </Button>
                        </div>
                    </CardContent>
                </Card>
            </div>

            {/* Server List Grid */}
            <DndContext
                sensors={sensors}
                collisionDetection={closestCenter}
                onDragEnd={handleDragEnd}
            >
                <div className="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-6 w-full">
                    {servers.length === 0 && (
                        <div className="col-span-full flex flex-col items-center justify-center py-12 text-muted-foreground border-2 border-dashed border-muted rounded-xl">
                            <WifiOff className="w-12 h-12 mb-4 opacity-20" />
                            <p>No connections added yet.</p>
                        </div>
                    )}

                    <SortableContext
                        items={servers.map(s => s.ID)}
                        strategy={rectSortingStrategy}
                    >
                        {servers.map((s) => (
                            <SortableServerCard key={s.ID} server={s} />
                        ))}
                    </SortableContext>
                </div>
            </DndContext>
        </div>
    );
}

function SortableServerCard({ server }: { server: Server }) {
    const {
        attributes,
        listeners,
        setNodeRef,
        transform,
        transition,
        isDragging,
    } = useSortable({ id: server.ID });

    const style = {
        transform: CSS.Transform.toString(transform),
        transition,
        opacity: isDragging ? 0.5 : 1,
        zIndex: isDragging ? 50 : "auto",
    };

    const isConnected = server.Status === "Connected";
    const isConnecting = server.Status === "Connecting...";
    const [isRetrying, setIsRetrying] = useState(false);

    const handleRetry = async () => {
        setIsRetrying(true);
        try {
            await CheckServerConnection(server.ID);
        } finally {
            setIsRetrying(false);
        }
    };

    return (
        <div ref={setNodeRef} style={style} className="relative group min-w-0 h-full">
            <Card className="transition-all duration-200 hover:border-primary/50 h-full flex flex-col w-full min-w-[300px]">
                <CardHeader className="pb-3 flex flex-row items-start justify-between space-y-0 relative">
                    <div className="flex flex-col gap-1 mr-2 overflow-hidden">
                        <CardTitle className="font-bold text-base truncate" title={server.Hostname || server.ID}>
                            {server.Hostname || "Unknown Host"}
                        </CardTitle>
                        <div className="text-xs text-muted-foreground font-mono truncate" title={server.Addr}>
                            {server.Addr}
                        </div>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                        {!server.Online && (
                            <Button variant="ghost" size="icon" className="h-6 w-6" onClick={handleRetry} disabled={isRetrying}>
                                <RefreshCw className={`w-3 h-3 ${isRetrying ? "animate-spin" : ""}`} />
                            </Button>
                        )}
                        <Badge variant={server.Online ? "success" : "secondary"} className={server.Online ? "" : "opacity-50"}>
                            {server.Online ? "Online" : "Offline"}
                        </Badge>
                        <button
                            {...attributes}
                            {...listeners}
                            className="text-muted-foreground/50 hover:text-foreground cursor-grab active:cursor-grabbing p-1 -mr-2"
                        >
                            <GripVertical className="w-4 h-4" />
                        </button>
                    </div>
                </CardHeader>

                <CardContent className="pb-3 flex-1">
                    <div className="flex items-center text-xs text-muted-foreground gap-2">
                        <Wifi className={`w-3 h-3 ${isConnected ? "text-green-500" : "opacity-50"}`} />
                        {isConnected ? "Streaming Active" : (server.Online ? "Ready to connect" : "Host Unreachable")}
                    </div>
                    <p className="text-xs text-destructive mt-2 font-medium">{(server.Status !== "Idle" && server.Status !== "Connected") ? server.Status : " "}</p>
                </CardContent>

                <CardFooter className="flex justify-between pt-3 border-t border-border/50 bg-muted/20">
                    <div className="flex gap-2">
                        <Button
                            variant={isConnected ? "destructive" : "default"}
                            size="sm"
                            disabled={isConnecting || !server.Online}
                            onClick={() => isConnected ? DisconnetFromAudioServer(server.ID) : ConnectToAudioServer(server.ID)}
                            className="w-24"
                        >
                            {isConnected ? "Disconnect" : (isConnecting ? "..." : "Connect")}
                        </Button>
                    </div>

                    <div className="flex gap-1">
                        <ServerSettingsDialog server={server} />
                        <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 hover:text-destructive"
                            onClick={() => RemoveServer(server.ID)}
                        >
                            <Trash2 className="w-4 h-4" />
                        </Button>
                    </div>
                </CardFooter>
            </Card>
        </div>
    );
}

export default ClientPage;