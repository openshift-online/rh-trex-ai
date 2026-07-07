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
  FormSelect,
  FormSelectOption,
  PageSection,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';

const RELATIONSHIP_TYPES = ['has_one', 'has_many', 'belongs_to', 'many_to_many'];

const RelationshipCreatePage: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const initialProjectId = params.get('project_id') || '';

  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [projectId, setProjectId] = React.useState(initialProjectId);
  const [sourceEntityId, setSourceEntityId] = React.useState('');
  const [targetEntityId, setTargetEntityId] = React.useState('');
  const [relationshipType, setRelationshipType] = React.useState('has_many');
  const [foreignKey, setForeignKey] = React.useState('');
  const [entities, setEntities] = React.useState<Array<Record<string, unknown>>>([]);

  React.useEffect(() => {
    if (!projectId) return;
    const api = createAPIClient();
    api.entityDefinitions.list({ projectId, size: 100 })
      .then((resp) => setEntities(resp.items))
      .catch(() => setEntities([]));
  }, [projectId]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    const body: Record<string, unknown> = {
      project_id: projectId,
      source_entity_id: sourceEntityId,
      target_entity_id: targetEntityId,
      relationship_type: relationshipType,
    };
    if (foreignKey) body.foreign_key = foreignKey;

    try {
      const api = createAPIClient();
      const created = await api.relationships.create(body);
      history.push(`/trex-console/relationships/${created.id}`);
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
            onClick={() => history.push('/trex-console/relationships')}
            component="button"
          >
            Relationships
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Create Relationship</Title>
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
          <FormGroup label="Source Entity" fieldId="field-source_entity_id" isRequired>
            {entities.length > 0 ? (
              <FormSelect
                id="field-source_entity_id"
                value={sourceEntityId}
                onChange={(_e, val) => setSourceEntityId(val)}
              >
                <FormSelectOption key="" value="" label="Select source entity..." />
                {entities.map((e) => (
                  <FormSelectOption
                    key={String(e.id)}
                    value={String(e.id)}
                    label={`${String(e.kind_name)} (${String(e.id).substring(0, 8)}...)`}
                  />
                ))}
              </FormSelect>
            ) : (
              <TextInput
                id="field-source_entity_id"
                value={sourceEntityId}
                onChange={(_e, val) => setSourceEntityId(val)}
                placeholder="Enter Source Entity ID"
                isRequired
              />
            )}
          </FormGroup>
          <FormGroup label="Target Entity" fieldId="field-target_entity_id" isRequired>
            {entities.length > 0 ? (
              <FormSelect
                id="field-target_entity_id"
                value={targetEntityId}
                onChange={(_e, val) => setTargetEntityId(val)}
              >
                <FormSelectOption key="" value="" label="Select target entity..." />
                {entities.map((e) => (
                  <FormSelectOption
                    key={String(e.id)}
                    value={String(e.id)}
                    label={`${String(e.kind_name)} (${String(e.id).substring(0, 8)}...)`}
                  />
                ))}
              </FormSelect>
            ) : (
              <TextInput
                id="field-target_entity_id"
                value={targetEntityId}
                onChange={(_e, val) => setTargetEntityId(val)}
                placeholder="Enter Target Entity ID"
                isRequired
              />
            )}
          </FormGroup>
          <FormGroup label="Relationship Type" fieldId="field-relationship_type" isRequired>
            <FormSelect
              id="field-relationship_type"
              value={relationshipType}
              onChange={(_e, val) => setRelationshipType(val)}
            >
              {RELATIONSHIP_TYPES.map((t) => (
                <FormSelectOption key={t} value={t} label={t} />
              ))}
            </FormSelect>
          </FormGroup>
          <FormGroup label="Foreign Key (optional)" fieldId="field-foreign_key">
            <TextInput
              id="field-foreign_key"
              value={foreignKey}
              onChange={(_e, val) => setForeignKey(val)}
              placeholder="Auto-derived if omitted"
            />
          </FormGroup>
          <ActionGroup>
            <Button type="submit" variant="primary" isLoading={submitting} isDisabled={submitting}>
              Create
            </Button>
            <Button
              variant="link"
              onClick={() => history.push('/trex-console/relationships')}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
};

export default RelationshipCreatePage;
