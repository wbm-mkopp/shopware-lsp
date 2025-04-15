import * as path from 'path';
import * as fs from 'fs';
import * as vscode from 'vscode';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
  RevealOutputChannelOn
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  // Create output channel for the language server
  const outputChannel = vscode.window.createOutputChannel("Symfony Service LSP");
  context.subscriptions.push(outputChannel);

  async function startClient() {
    if (client) {
      await client.stop();
      client = undefined;
    }

    // Get the server path from settings or use default
    let serverPath = '';
    // If no custom path is provided, use the bundled server
    // For development, we'll look for the server in the parent directory
    const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || '';
    const possiblePaths = [
      // When installed as extension
      context.asAbsolutePath(path.join('..', 'shopware-lsp')),
      // When running from source
      path.join(workspaceRoot, '..', 'shopware-lsp'),
      // When in the same directory
      path.join(workspaceRoot, 'shopware-lsp')
    ];

    for (const p of possiblePaths) {
      if (fs.existsSync(p)) {
        serverPath = p;
        break;
      }
    }

    if (!serverPath) {
      vscode.window.showErrorMessage('Could not find Symfony Service LSP server. Please set the path in settings.');
      return;
    }

    // Define server options
    const serverOptions: ServerOptions = {
      command: serverPath,
      args: [],
      transport: TransportKind.stdio
    };

    // Define client options
    const clientOptions: LanguageClientOptions = {
      documentSelector: [
        { scheme: 'file', language: 'php' },
        { scheme: 'file', language: 'xml' }
      ],
      synchronize: {
        fileEvents: vscode.workspace.createFileSystemWatcher('**/*.{php,xml}')
      },
      // Add output configuration
      outputChannel: outputChannel,
      traceOutputChannel: outputChannel,
      revealOutputChannelOn: RevealOutputChannelOn.Error
    };

    // Show output channel on start
    outputChannel.appendLine(`Starting Symfony Service Language Server at ${serverPath}`);
    outputChannel.show();

    // Create and start the client
    client = new LanguageClient(
      'symfonyServiceLSP',
      'Symfony Service Language Server',
      serverOptions,
      clientOptions
    );

    // Register notification handler for service counts
    client.start().then(() => {
      client!.onNotification('symfony/serviceCount', (params: { serviceCount: number, aliasCount: number, total: number }) => {
        // Update status bar with service count
        outputChannel.appendLine(`Found ${params.serviceCount} services, ${params.aliasCount} aliases (${params.total} total)`); 
        vscode.window.setStatusBarMessage(`Symfony: ${params.total} services`, 10000);
      });
    }).catch((err: Error) => {
      outputChannel.appendLine(`Error registering notification handler: ${err}`);
    });

    vscode.window.showInformationMessage('Symfony Service LSP activated');
  }

  // Start client on activation and await it
  await startClient();

  // Register restart command
  context.subscriptions.push(vscode.commands.registerCommand('symfonyServiceLSP.restart', async () => {
    await startClient();
    vscode.window.showInformationMessage('Symfony Service LSP restarted');
  }));
}

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  
  // Add a timeout to ensure the server has time to respond
  return new Promise<void>((resolve) => {
    // Try to stop the client gracefully
    const stopPromise = client!.stop();
    
    // Set a timeout in case the stop hangs
    const timeout = setTimeout(() => {
      console.log('Client stop timed out, forcing resolution');
      resolve();
    }, 2000); // 2 second timeout
    
    // Handle normal completion
    stopPromise.then(() => {
      clearTimeout(timeout);
      resolve();
    }).catch(error => {
      console.error('Error stopping client:', error);
      clearTimeout(timeout);
      resolve(); // Resolve anyway to prevent VSCode from hanging
    });
  });
}
