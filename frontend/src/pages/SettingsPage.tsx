import { useEffect, useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { FolderOpen, Loader2 } from "lucide-react";
import { GithubIcon } from "@/components/GithubIcon";
import { useToast } from "@/components/ui/use-toast";
import { GetServerConfig, SaveServerConfig, GetClientConfig, SaveClientConfig, OpenConfigDir, GetConfigDir } from "../../wailsjs/go/main/App";
import { main, pcmresample } from "../../wailsjs/go/models";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { ResampleSettings } from "@/components/ResampleSettings";

export default function SettingsPage() {
    const { toast } = useToast();
    const [serverCfg, setServerCfg] = useState(new main.ServerConfig({ port: "8080" }));
    const [clientCfg, setClientCfg] = useState(new main.ClientConfig({
        defaultResampleOpts: new pcmresample.Options({
            Method: 2, Quality: 20, MaxQuality: 100, MaxThreads: 0, IncludeLFEInDownmix: true
        })
    }));
    const [configPath, setConfigPath] = useState("");
    const [loading, setLoading] = useState(false);

    useEffect(() => {
        GetConfigDir().then(setConfigPath);
        GetServerConfig().then(c => setServerCfg(new main.ServerConfig(c)));
        GetClientConfig().then(c => setClientCfg(new main.ClientConfig(c)));
    }, []);

    const handleSaveServer = async () => {
        setLoading(true);
        try {
            let cfg = serverCfg;
            if (cfg.port === "") {
                cfg = new main.ServerConfig({ ...cfg, port: "8080" });
                setServerCfg(cfg);
            }
            await SaveServerConfig(cfg);
            toast({ title: "Settings Saved", description: "Server configuration updated." });
        } catch (e: any) {
            toast({ variant: "destructive", title: "Error", description: e.toString() });
        } finally {
            setLoading(false);
        }
    };

    const handleSaveClient = async () => {
        setLoading(true);
        try {
            await SaveClientConfig(clientCfg);
            toast({ title: "Settings Saved", description: "Client configuration updated." });
        } catch (e: any) {
            toast({ variant: "destructive", title: "Error", description: e.toString() });
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="flex flex-col space-y-6 h-full w-full">
            <div>
                <h1 className="text-2xl font-bold tracking-tight">Settings</h1>
                <p className="text-muted-foreground">Manage application preferences.</p>
            </div>

            <Tabs defaultValue="general" className="w-full">
                <TabsList className="grid w-full grid-cols-3">
                    <TabsTrigger value="general">General</TabsTrigger>
                    <TabsTrigger value="server">Sender Mode</TabsTrigger>
                    <TabsTrigger value="client">Receiver Mode</TabsTrigger>
                </TabsList>

                <TabsContent value="general" className="space-y-2">
                    <Card>
                        <CardHeader>
                            <CardTitle>Configuration Storage</CardTitle>
                            <CardDescription>Location of your setting files.</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <div className="flex items-center gap-4">
                                <Input value={configPath} readOnly className="font-mono text-xs" />
                                <Button onClick={OpenConfigDir} variant="outline">
                                    <FolderOpen className="mr-2 h-4 w-4" /> Open
                                </Button>
                            </div>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardHeader>
                            <CardTitle>Updates</CardTitle>
                            <CardDescription>Check the GitHub for updates</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <Button className="" onClick={() => { BrowserOpenURL("https://github.com/ElTheLedge/Audio-Over-IP") }}>
                                <GithubIcon className="mr-1" />@ElTheLedge/Audio-Over-IP
                            </Button>
                        </CardContent>
                    </Card>
                </TabsContent>

                <TabsContent value="server">
                    <Card>
                        <CardHeader>
                            <CardTitle>Network</CardTitle>
                            <CardDescription>Configure how the audio server listens for connections.</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <div className="grid gap-2">
                                <label className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70">
                                    Listening Port
                                </label>
                                <Input
                                    type="number"
                                    value={serverCfg.port}
                                    onChange={(e) => {
                                        setServerCfg(new main.ServerConfig({ ...serverCfg, port: e.target.value }))
                                    }}
                                    placeholder="8080"
                                />
                                <p className="text-[0.8rem] text-muted-foreground">
                                    The port on which the audio stream will be broadcast. Requires server restart to apply.
                                </p>
                            </div>
                            <Button onClick={handleSaveServer} disabled={loading}>
                                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                Save Changes
                            </Button>
                        </CardContent>
                    </Card>
                </TabsContent>

                <TabsContent value="client">
                    <Card>
                        <CardHeader>
                            <CardTitle>Default Audio Processing</CardTitle>
                            <CardDescription>These settings apply to all new server connections.</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-6">
                            <ResampleSettings
                                opts={clientCfg.defaultResampleOpts}
                                onChange={(opts) => setClientCfg(new main.ClientConfig({ ...clientCfg, defaultResampleOpts: opts }))}
                            />
                            <Button onClick={handleSaveClient} disabled={loading}>
                                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                                Save Changes
                            </Button>
                        </CardContent>
                    </Card>
                </TabsContent>
            </Tabs>
        </div>
    );
}


