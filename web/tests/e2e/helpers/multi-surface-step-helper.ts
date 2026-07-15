import { expect, type Page, type TestInfo } from '@playwright/test';
import { writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';

export type Verification = {
  spec: string;
  check: () => Promise<void>;
};

export type Surface = {
  id: string;
  name: string;
  page: Page;
  verifications: Verification[];
};

export type StepOptions = {
  description: string;
  surfaces: Surface[];
};

type DocumentedSurface = {
  name: string;
  image: string;
  specs: string[];
};

type DocumentedStep = {
  description: string;
  surfaces: DocumentedSurface[];
};

export class MultiSurfaceStepHelper {
  private stepCount = 0;
  private steps: DocumentedStep[] = [];
  private title = '';
  private description = '';

  constructor(private testInfo: TestInfo) {}

  setMetadata(title: string, description: string) {
    this.title = title;
    this.description = description;
  }

  async step(id: string, options: StepOptions) {
    if (options.surfaces.length === 0) throw new Error(`E2E step ${id} has no surfaces`);
    const index = String(this.stepCount++).padStart(3, '0');
    const slug = id.replace(/_/g, '-');
    const documentedSurfaces: DocumentedSurface[] = [];

    console.log(`\n[E2E step] ${id}: ${options.description}`);
    for (const surface of options.surfaces) {
      if (surface.verifications.length === 0) throw new Error(`E2E surface ${surface.id} has no verifications`);
      console.log(`[E2E surface] ${surface.name}`);
      for (const verification of surface.verifications) {
        console.log(`[E2E check] ${surface.name}: ${verification.spec}`);
        await verification.check();
        console.log(`[E2E pass] ${surface.name}: ${verification.spec}`);
      }

      const filename = `${index}-${slug}--${surface.id}.png`;
      await expect(surface.page).toHaveScreenshot(filename);
      documentedSurfaces.push({
        name: surface.name,
        image: `./screenshots/${filename}`,
        specs: surface.verifications.map((verification) => verification.spec)
      });
    }

    this.steps.push({ description: options.description, surfaces: documentedSurfaces });
  }

  generateDocs() {
    const title = this.title || this.testInfo.title;
    let content = `# Test: ${title}\n\n`;
    if (this.description) content += `${this.description}\n\n`;

    for (const step of this.steps) {
      content += `## ${step.description}\n\n`;
      for (const surface of step.surfaces) {
        content += `### ${surface.name}\n\n`;
        content += `![${step.description} — ${surface.name}](${surface.image})\n\n`;
        content += `**Verifications for ${surface.name}:**\n`;
        for (const spec of surface.specs) content += `- [x] ${spec}\n`;
        content += '\n';
      }
      content += '---\n\n';
    }

    writeFileSync(join(dirname(this.testInfo.file), 'README.md'), `${content.trimEnd()}\n`);
  }
}
