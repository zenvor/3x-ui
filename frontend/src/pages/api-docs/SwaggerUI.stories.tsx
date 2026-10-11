import type { Meta, StoryObj } from '@storybook/react-vite';
import SwaggerUI from 'swagger-ui-react';
import { expect, within } from 'storybook/test';

const meta = {
  title: 'Pages/API Docs/SwaggerUI',
  component: SwaggerUI,
  tags: ['autodocs'],
  args: {
    spec: {
      openapi: '3.0.3',
      info: {
        title: 'Markdown compatibility',
        version: '1.0.0',
        description: [
          '## Authentication guide',
          '',
          '- Use **scoped tokens**',
          '- Read the [API reference](https://example.test/reference)',
          '',
          '```http',
          'Authorization: Bearer example',
          '```',
          '',
          '<script>window.markdownInjected = true</script>',
          '<span onclick="window.markdownInjected = true">Safe HTML</span>',
        ].join('\n'),
      },
      paths: {},
    },
  },
} satisfies Meta<typeof SwaggerUI>;

export default meta;
type Story = StoryObj<typeof meta>;

// Exercise Markdown output after removing remarkable's sprintf-js chain.
// Panel styling is supplied separately by ApiDocsPage.
export const MarkdownCompatibility: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(
      await canvas.findByRole('heading', { name: 'Authentication guide' }),
    ).toBeInTheDocument();
    await expect(canvas.getByText('scoped tokens').tagName).toBe('STRONG');
    await expect(canvas.getByRole('link', { name: 'API reference' })).toHaveAttribute(
      'href',
      'https://example.test/reference',
    );
    await expect(
      canvas.getByText('Authorization: Bearer example').closest('pre'),
    ).toBeInTheDocument();
    await expect(canvasElement.querySelector('script, [onclick]')).toBeNull();
  },
};
