import * as React from 'react';
import { useHistory, useLocation } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Form,
  FormGroup,
  PageSection,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';

const EntityDefinitionCreatePage: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const initialProjectId = params.get('project_id') || '';

  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [description, setDescription] = React.useState('');
  const [kindName, setKindName] = React.useState('');
  const [pluralOverride, setPluralOverride] = React.useState('');
  const [projectId, setProjectId] = React.useState(initialProjectId);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    const body: Record<string, unknown> = {
      kind_name: kindName,
      project_id: projectId,
    };
    if (description) body.description = description;
    if (pluralOverride) body.plural_override = pluralOverride;

    try {
      const api = createAPIClient();
      const created = await api.entityDefinitions.create(body);
      history.push(`/trex-console/entity-definitions/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/entity-definitions')}
            component="button"
          >
            Entity Definitions
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Create Entity Definition</Title>
      </PageSection>
      <PageSection>
        {error && <Alert variant="danger" title="Error" isInline>{error}</Alert>}
        <Form onSubmit={handleSubmit} style={{ maxWidth: 600 }}>
          <FormGroup label="Project ID" fieldId="field-project_id" isRequired>
            <TextInput
              id="field-project_id"
              value={projectId}
              onChange={(_e, val) => setProjectId(val)}
              placeholder="Enter Project ID"
              isRequired
              isDisabled={!!initialProjectId}
            />
          </FormGroup>
          <FormGroup label="Kind Name" fieldId="field-kind_name" isRequired>
            <TextInput
              id="field-kind_name"
              value={kindName}
              onChange={(_e, val) => setKindName(val)}
              placeholder="e.g. User, Product, Order"
              isRequired
            />
          </FormGroup>
          <FormGroup label="Description" fieldId="field-description">
            <TextInput
              id="field-description"
              value={description}
              onChange={(_e, val) => setDescription(val)}
              placeholder="Describe this entity"
            />
          </FormGroup>
          <FormGroup label="Plural Override" fieldId="field-plural_override">
            <TextInput
              id="field-plural_override"
              value={pluralOverride}
              onChange={(_e, val) => setPluralOverride(val)}
              placeholder="Auto-derived if omitted"
            />
          </FormGroup>
          <ActionGroup>
            <Button type="submit" variant="primary" isLoading={submitting} isDisabled={submitting}>
              Create
            </Button>
            <Button
              variant="link"
              onClick={() => history.push('/trex-console/entity-definitions')}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
};

export default EntityDefinitionCreatePage;
