<?php declare(strict_types=1);

namespace Shopware\Core\Content\Product\Test;

use Shopware\Core\Framework\Log\Package;
use Symfony\Component\HttpFoundation\Request as SymfonyRequest;
use Shopware\Core\Content\Product\Service\ProductLoader as Loader;
use Doctrine\DBAL\Connection;

#[Package('inventory')]
class ProductTest
{
    private readonly SymfonyRequest $request;
    private readonly Loader $productLoader;
    private readonly Connection $connection;

    /**
     * @internal
     */
    public function __construct(
        private readonly string $testId
    ) {
        $this->request = new SymfonyRequest();
        $this->productLoader = new Loader('test');
        $this->connection = new Connection([]);
    }

    public function getLoader(): Loader
    {
        return $this->productLoader;
    }

    protected function validateRequest(SymfonyRequest $request): bool
    {
        return $request->isMethod('GET');
    }

    private function getConnection(): Connection
    {
        return $this->connection;
    }
}
