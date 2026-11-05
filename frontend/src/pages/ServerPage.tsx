import { Switch } from "@/components/ui/switch"
import { useEffect, useState } from "react"
import { Events } from "@wailsio/runtime";
import { DisableServer, EnableServer, GetServerConfig, GetServerEnabled } from "../../bindings/AuOvIP/app";
import { ServerStats } from "../../bindings/AuOvIP/models";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Activity, Users, Radio, Power } from "lucide-react";
import { cn } from "@/lib/utils";

function ServerPage() {
    const [isServerOn, setIsServerOn] = useState(false);
    const [serverStats, setServerStats] = useState<ServerStats>({ connectedClients: 0, bandwidth: 0 });
    const [listenPort, setListenPort] = useState("8080")

    useEffect(() => {
        GetServerConfig().then((conf)=> {
            setListenPort(conf.port)
        })
        GetServerEnabled().then((isOn) => {
            setIsServerOn(isOn);
        });
        const unsubscribers: Array<() => void> = [];
        unsubscribers.push(Events.On("updateServerStatus", (e) => {
            setIsServerOn(e.data);
        }));
        unsubscribers.push(Events.On("updateServerStats", (e) => {
            setServerStats(e.data);
        }));
        unsubscribers.push(Events.On("updateServerPagePortDisplay", (e) => {
            setListenPort(e.data);
        }));
        return () => {
            for (const off of unsubscribers) off();
        };
    }, []);

    return (
        <div className="flex flex-col space-y-8 h-full w-full">
            {/* Header */}
            <div>
                <h1 className="text-2xl font-bold tracking-tight">Sender Mode</h1>
                <p className="text-muted-foreground">Broadcast your system audio to connected receivers.</p>
            </div>

            {/* Main Control Card */}
            <Card className={cn(
                "border-l-4 transition-all duration-500",
                isServerOn ? "border-l-green-500 bg-green-500/5" : "border-l-border"
            )}>
                <CardContent className="p-8 flex items-center justify-between">
                    <div className="flex items-center gap-6">
                        <div className={cn(
                            "p-4 rounded-full transition-colors duration-500",
                            isServerOn ? "bg-green-500 text-white shadow-lg shadow-green-500/20" : "bg-muted text-muted-foreground"
                        )}>
                            <Power className="w-8 h-8" />
                        </div>
                        <div className="flex flex-col">
                            <h2 className="text-xl font-semibold">
                                {isServerOn ? "Broadcasting Active" : "Server Offline"}
                            </h2>
                            <p className="text-muted-foreground">
                                {isServerOn ? "Audio is being captured and streamed." : "Start the server to begin streaming."}
                            </p>
                        </div>
                    </div>
                    
                    <div className="flex items-center gap-4">
                        <span className="text-sm font-medium uppercase tracking-wider text-muted-foreground">
                            {isServerOn ? "On" : "Off"}
                        </span>
                        <Switch
                            className="scale-150 data-[state=checked]:bg-green-500"
                            checked={isServerOn}
                            onCheckedChange={(isChecked) => {
                                if (isChecked) {
                                    EnableServer();
                                } else {
                                    DisableServer();
                                }
                            }}
                        />
                    </div>
                </CardContent>
            </Card>

            {/* Stats */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                <StatsCard 
                    title="Clients" 
                    value={serverStats.connectedClients.toString()} 
                    icon={<Users className="w-4 h-4" />}
                    desc="Active Connections"
                />
                <StatsCard 
                    title="Bandwidth" 
                    value={`${serverStats.bandwidth} kbps`} 
                    icon={<Activity className="w-4 h-4" />}
                    desc="Total Outbound"
                />
                <StatsCard 
                    title="Port" 
                    value={listenPort} 
                    icon={<Radio className="w-4 h-4" />}
                    desc="Listening Port"
                />
            </div>
        </div>
    );
}

function StatsCard({ title, value, icon, desc }: { title: string, value: string, icon: React.ReactNode, desc: string }) {
    return (
        <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">
                    {title}
                </CardTitle>
                {icon}
            </CardHeader>
            <CardContent>
                <div className="text-2xl font-bold">{value}</div>
                <p className="text-xs text-muted-foreground mt-1">
                    {desc}
                </p>
            </CardContent>
        </Card>
    )
}

export default ServerPage
