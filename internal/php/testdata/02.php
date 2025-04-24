<?php declare(strict_types=1);

namespace Shopware\Core\Content\Product\Service;

use Shopware\Core\Framework\Log\Package;
use Symfony\Component\HttpFoundation\Request;

#[Package('inventory')]
class ProductLoader
{
    private readonly Request $request;

    /**
     * @internal
     */
    public function __construct(
        private readonly string $productRepository
    ) {
        $this->request = new Request();
    }

    public function load(string $id): array
    {
        return ['id' => $id];
    }

    protected function validateId(string $id): bool
    {
        return strlen($id) > 0;
    }

    private function getRepository(): string
    {
        return $this->productRepository;
    }
}
